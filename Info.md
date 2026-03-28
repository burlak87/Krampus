krampus-messenger/
├── cmd/
│   └── app/                    # Ваш main.go без изменений
│       └── main.go
├── internal/                   # Модули по доменам (internal/ изоляция)
│   ├── bootstrap/              # НОВОЕ: wiring services (NewServices(cfg))
│   │   └── services.go
│   ├── user/                   # User + auth логика
│   │   ├── domain/             # user.go, session.go, twofa.go
│   │   ├── service/            # user_service.go, refreshToken_service.go
│   │   ├── storage/            # user_storage.go (psql), redis/session.go
│   │   └── adapters/           # handlers/user_handler.go
│   ├── message/                # Messages + file storage
│   │   ├── domain/             # message.go
│   │   ├── service/            # message_service.go, rateLimiter_service.go
│   │   ├── storage/            # psql/message_storage.go, file/file_storage.go
│   │   └── adapters/           # websocket/client.go, manager.go
│   ├── chat/                   # Rooms + rooms
│   │   ├── domain/             # room.go
│   │   ├── service/            # room_service.go, userClient_service.go
│   │   ├── storage/            # room_storage.go (psql/redis cache)
│   │   └── adapters/           # handlers/chat_routes.go
│   └── auth/                   # 2FA + refresh
│       ├── domain/             # code.go
│       ├── service/            # twofa_service.go
│       ├── storage/            # twofa_storage.go (psql/redis)
│       └── adapters/           # handlers/twofa_handler.go, refresh_handler.go
├── pkg/                        # Shared (ваш код без изменений)
│   ├── config/                 # config.go
│   ├── logging/                # logging.go
│   ├── apperror/               # apperror.go
│   ├── server/                 # server.go, middleware/auth.go, routing.go
│   └── client-database/        # postgresql/postgresql.go
├── storage/psql/sqlc/          # Shared DB (ваш database/ → сюда)
│   ├── db.go
│   ├── models.go
│   └── querier.go
└── go.mod                      # Единственный, + internal/ модули

<!--krampus-messenger/  # go mod init krampus-messenger
├── cmd/
│   └── app/
│       └── main.go  # Init config, storage, wire modules → server
├── internal/
│   ├── bootstrap/   # wiring.go: NewServices() композирует модули
│   │   └── services.go
│   ├── user/        # Полная копия вашей user логики
│   │   ├── domain/          # user.go, session.go (из paste.txt)
│   │   ├── usecase/         # user_service.go
│   │   ├── repository/      # user_repository.go (psql), redis/session.go
│   │   ├── adapters/        # handlers/user_handler.go
│   │   └── interfaces.go    # UserService interface (экспорт)
│   ├── message/     # message + websocket
│   │   ├── domain/          # message.go, room.go
│   │   ├── usecase/         # message_service.go, rateLimiter
│   │   ├── repository/      # psql/file storage
│   │   ├── adapters/        # websocket/{client,manager,server}.go
│   │   └── interfaces.go
│   ├── auth/        # refresh + 2fa
│   │   ├── domain/          # twofa.go
│   │   ├── usecase/         # refreshToken_service.go, twofa_service.go
│   │   ├── repository/      # refresh_repository.go, twofa_repository.go
│   │   └── adapters/        # handlers/{refresh_token,twofa}_handler.go
│   └── chat/        # rooms + websocket routes
│       ├── domain/          # room.go
│       ├── usecase/         # room_service.go
│       ├── repository/
│       └── adapters/        # adapters/handlers.go (routes)
├── pkg/                 # Shared (из paste.txt, public/exposed)
│   ├── config/
│   ├── logging/
│   ├── apperror/
│   ├── server/          # server.go (Gin setup)
│   └── client-database/ # postgresql
└── go.mod  # Только один! require только внешние (gin, pgx, redis)-->

cmd
├── app
|    └── main.go:
```
package main

import (
	"context"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"crampus/internal/adapters/rest"
	"crampus/internal/service"
	"crampus/internal/storage/psql"
	database "crampus/internal/storage/psql/sqlc"
	"crampus/pkg/client-database/postgresql"
	"crampus/pkg/config"
	"crampus/pkg/logging"
	"crampus/pkg/server"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	logging.Init()
	logger := logging.GetLogger()
	logger.Infoln("Starting application")

	cfg := config.GetConfig()
	overrideConfigFromEnv(cfg)
	logger.Infof("Environment: %s", cfg.Env)
	logger.Infof("DB CONFIG: Host=%s, Port=%s, Database=%s, Username=%s", cfg.Host, cfg.Port, cfg.Database, cfg.Username)

	dsn := getDSN(cfg)
	logger.Infof("Using DSN: %s", dsn)

	jwtSecret := getJWTSecret()
	logger.Infof("JWT secret configured: %s", maskSecret(jwtSecret))

	postgresSQLClient, err := postgresql.NewClient(context.TODO(), 15, cfg.StorageConfig)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer postgresSQLClient.Close()

	queries := database.New(postgresSQLClient)
	storage := psql.NewStorage(queries)

	userService := service.NewUser(storage, jwtSecret)
	userHandler := rest.NewUsersHandler(userService, logger)

	port := getPort()

	serverCfg := server.Config{
		Port:         port,
		Mode:         cfg.Env,
		CorsOrigins:  []string{"*"},
		CorsEnabled:  true,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	srv := server.NewServer(serverCfg, logger.Logger)

	srv.RegisterRoutes(func(engine *gin.Engine) {
		api := engine.Group("/api")
		{
			auth := api.Group("/auth")
			userHandler.RegisterRoutes(auth, jwtSecret)
		}

		engine.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "ok",
				"time":   time.Now().UTC(),
			})
		})
	})

	logger.Infof("Starting server on port %s", serverCfg.Port)
	if err := srv.Start(); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}

func overrideConfigFromEnv(cfg *config.Config) {
	if env := os.Getenv("APP_ENV"); env != "" {
		cfg.Env = env
	}
	if host := os.Getenv("DB_HOST"); host != "" {
		cfg.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		cfg.Port = port
	}
	if db := os.Getenv("DB_NAME"); db != "" {
		cfg.Database = db
	}
	if user := os.Getenv("DB_USER"); user != "" {
		cfg.Username = user
	}
	if pass := os.Getenv("DB_PASSWORD"); pass != "" {
		cfg.Password = pass
	}
}

func getJWTSecret() string {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret
	}
	return "my-secret-key"
}

func maskSecret(secret string) string {
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:2] + "****" + secret[len(secret)-2:]
}

func getPort() string {
	if port := os.Getenv("APP_PORT"); port != "" {
		return port
	}
	return "8888"
}

func getDSN(cfg *config.Config) string {
	return "postgresql://" + cfg.Username + ":" + cfg.Password + "@" + cfg.Host + ":" + cfg.Port + "/" + cfg.Database + "?sslmode=disable&pool_max_conns=20"
}
```
├── web
    └── main.go
internal
├── adapters
| ├── refreshToken
| | └── handler.go:
```
package refreshToken

import (
	"krampus/internal/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RefreshTokenService interface {
	// SaveSession() ()
	UserRefresh(token string) (domain.TokenResponse, error)
	// UserSendEmailCode(tempToken string) error
}

type refreshTokenHandler struct {
	refreshTokenService RefreshTokenService
	logger              *logging.Logger
}

func NewRefreshTokenHandler(s RefreshTokenService, l *logging.Logger) *refreshTokenHandler {
	return &refreshTokenHandler{
		refreshTokenService: s,
		logger:              l,
	}
}

func (h *refreshTokenHandler) refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	token, err := h.refreshTokenService.UserRefresh(req.RefreshToken)
	if err != nil {
		h.logger.Error("Failed to refresh user: " + err.Error())
		appErr, ok := err.(*apperror.AppError)
		if ok {
			c.JSON(http.StatusUnauthorized, appErr)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Failed to refresh token",
			})
		}
		return
	}

	c.JSON(http.StatusOK, token)
}

// func (h *refreshTokenHandler) sendEmailToken(c *gin.Context) {
// 	var req struct {
// 		TempToken string `json:"temp_token"`
// 	}
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		h.logger.Error("Failed to bind JSON: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Invalid request body",
// 		})
// 		return
// 	}

// 	err := h.refreshTokenService.UserSendEmailCode(req.TempToken)
// 	if err != nil {
// 		h.logger.Error("Failed to send verification code: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Failed to send verification code",
// 		})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"success": true,
// 	})
// }
```
| ├── twofa
| | └── handler.go:
```
package twofa

import (
	"krampus/internal/domain"
	"krampus/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TwoFAService interface {
	VerifyCode(code domain.Code) (domain.TokenResponse, error)
	EnableTwoFA(userID int64) error
	// DisableTwoFA(userID int64, passqord string) error
}

type twoFAHandler struct {
	twoFAService TwoFAService
	logger       *logging.Logger
}

func NewTwoFAHandler(s TwoFAService, l *logging.Logger) *twoFAHandler {
	return &twoFAHandler{
		twoFAService: s,
		logger:       l,
	}
}

func (h *twoFAHandler) VerifyCode(c *gin.Context) {
	var code domain.Code
	if err := c.ShouldBindJSON(&code); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	tokenRes, err := h.twoFAService.VerifyCode(code)
	if err != nil {
		h.logger.Error("Failed to verify code: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Verification failed",
		})
		return
	}

	c.JSON(http.StatusOK, tokenRes)
}

func (h *twoFAHandler) EnableTwoFA(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	err := h.twoFAService.EnableTwoFA(userID.(int64))
	if err != nil {
		h.logger.Error("Failed to enable 2FA: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to enable 2FA",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// func (h *twoFAHandler) DisableTwoFA(c *gin.Context) {
// 	userID, exists := c.Get("userID")
// 	if !exists {
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Unauthorized",
// 		})
// 		return
// 	}

// 	var req domain.TwoFaToggleRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		h.logger.Error("Failed to bind JSON: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Invalid request body",
// 		})
// 		return
// 	}

// 	err := h.twoFAService.DisableTwoFA(userID.(int64), req.Password)
// 	if err != nil {
// 		h.logger.Error("Failed to enable 2FA: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Failed to enable 2FA",
// 		})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"success": true,
// 	})
// }
```
| ├── user
| | └── handler.go:
```
package user

import (
	"krampus/internal/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserService interface {
	UserRegister(users domain.User) (domain.User, error)
	UserLogin(users domain.User) (domain.TokenResponse, domain.TwoFaCodes, error)
	UserLogout(userID int64, password string) error
}

type userHandler struct {
	userService UserService
	logger      *logging.Logger
}

func NewUserHandler(s UserService, l *logging.Logger) *userHandler {
	return &userHandler{
		userService: s,
		logger:      l,
	}
}

func (h *userHandler) signUp(c *gin.Context) {
	var user domain.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	createdUser, err := h.userService.UserRegister(user)
	if err != nil {
		h.logger.Error("Failed to register user: " + err.Error())
		appErr, ok := err.(*apperror.AppError)
		if ok {
			c.JSON(http.StatusBadRequest, appErr)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to register user",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, createdUser)
}

func (h *userHandler) signIn(c *gin.Context) {
	var user domain.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	accessToken, tempToken, err := h.userService.UserLogin(user)
	if err != nil {
		h.logger.Error("Failed to login user: " + err.Error())
		appErr, ok := err.(*apperror.AppError)
		if ok {
			c.JSON(http.StatusUnauthorized, appErr)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication failed",
			})
		}
		return
	}

	if tempToken.RequiresTwoFa {
		c.JSON(http.StatusOK, tempToken)
		return
	}

	c.JSON(http.StatusOK, accessToken)
}

func (h *userHandler) logout(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	var req domain.TwoFaToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	err := h.userService.UserLogout(userID.(int64), req.Password)
	if err != nil {
		h.logger.Error("Failed to logout: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to logout",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
```
| ├── websocket
| | ├── client.go:
```
package websocket

import (
	"encoding/json"
	"fmt"
	"krampus/internal/domain"
	"krampus/pkg/apperror"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn   *websocket.Conn
	UserID string
	RoomID string
	Send   chan *domain.BaseMessage
	mu     sync.Mutex
}

func NewClient(conn *websocket.Conn, userID, roomID string) *Client {
	return &Client{
		conn:   conn,
		UserID: userID,
		RoomID: roomID,
		Send:   make(chan *domain.BaseMessage, 256),
	}
}

func (c *Client) ConnID() {
	return c.conn.RemoteAddr().String() + "-" + c.UserID
}

func (c *Client) Start() {
	go c.writePump()
	go c.readPump()
}

// func (c *Client) readPump() {
// 	defer c.conn.Close()
// 	for {
// 		var msg domain.BaseMessage
// 		err := c.conn.ReadJSON(&msg)
// 		if err != nil {
// 			break
// 		}
// 		msg.UserID = c.UserID
// 		msg.RoomID = c.RoomID
// 		// Process via services (stub log)
// 		println("Msg from", c.UserID, ":", string(msg.Payload))
// 	}
// }

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg domain.BaseMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS unexpected close: %v", err)
			}
			break
		}
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		msg.UserID = c.UserID
		msg.RoomID = c.RoomID
	}
}

// func (c *Client) writePump() {
// 	defer c.conn.Close()
// 	for msg := range c.Send {
// 		if err := c.conn.WriteJSON(msg); err != nil {
// 			break
// 		}
// 	}
// }

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.conn.Close()
				return
			}

			if message.Type == "ping" {
				c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
				continue
			}

			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteJSON(message); err != nil {
				log.Printf("WS write error: %v", err)
				return
			}
		}
	}
}

func (c *Client) SendError(appErr *apperror.AppError) {
	errorMsg := &domain.BaseMessage{
		Type:    "error",
		Payload: json.RawMessage(fmt.Sprintf(`{"code": "%s", "message": "%s}`, appErr.Code, appErr.Message)),
	}
	select {
	case c.Send <- errorMsg:
	default:
	}
}
```
| | ├── manager.go:
```
package websocket

import (
	"krampus/internal/domain"
	"log"
	"sync"
)

type ConnectionManager struct {
	users     sync.Map
	rooms     sync.Map
	userLocks sync.Map
}

type UserConnections struct {
	mu     sync.RWMutex
	conns  map[string]*Client
	userID string
}

type RoomSubscribers struct {
	mu     sync.RWMutex
	users  map[string]bool
	roomID string
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{}
}

func (m *ConnectionManager) Register(client *Client) error {
	userID, roomID := client.UserID, client.RoomID

	ucI, _ := m.users.LoadOrStore(userID, &UserConnections{
		conns:  make(map[string]*Client),
		userID: userID,
	})
	uc := ucI.(*ConnectionManager)

	lockI, _ := m.userLocks.LoadOrStore(userID, new(sync.Mutex))
	userLock := lockI.(*sync.Mutex)
	userLock.Lock()
	defer userLock.Unlock()

	uc.mu.Lock()
	uc.conns[client.ConnID()] = client
	uc.mu.Unlock()

	m.subscribeToRoom(roomID, userID)
	client.Start()

	log.Printf("WS registered: %s@%s (%s)", userID, roomID, client.ConnID())
	return nil
}

func (m *ConnectionManager) Unregister(client *Client) {
	userID, roomID := client.UserID, client.RoomID

	if ucI, ok := m.users.Load(userID); ok {
		uc := ucI.(*ConnectionManager)
		LockI, _ := m.userLocks.Load(userID)
		userLock := LockI.(*sync.Mutex)

		userLock.Lock()
		defer userLock.Unlock()

		uc.mu.Lock()
		delete(uc.conns, client.ConnID())
		uc.mu.Unlock()

		if len(uc.conns) == 0 {
			m.unsubscribeFromRoom(roomID, userID)
			// m.users.Delete(userID)
			// m.userLocks.Delete(userID)
		}
	}
}

func (m *ConnectionManager) BroadcastToRoom(roomID string, msg *domain.BaseMessage) error {
	subsI, ok := m.rooms.Load(roomID)
	if !ok {
		return nil
	}
	subs := subsI.(*RoomSubscribers)
	subs.mu.RLock()
	defer subs.mu.RUnlock()

	var wg sync.WaitGroup
	for userID := range subs.users {
		wg.Add(1)
		go func(targetUserID string) {
			defer wg.Done()
			m.SendToUser(targetUserID, msg)
		}(userID)
	}
	wg.Wait()

	return nil
}

func (m *ConnectionManager) SendToUser(userID string, msg *domain.BaseMessage) error {
	ucI, ok := m.users.Load(userID)
	if !ok {
		return nil
	}
	uc := ucI.(*UserConnections)
	LockI, _ := m.userLocks.LoadOrStore(userID, new(sync.Mutex))
	userLock := LockI.(*sync.Mutex)

	userLock.Lock()
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	defer userLock.Unlock()

	for _, client := range uc.conns {
		select {
		case client.Send <- msg:
		default:
			log.Printf("Dropped msg for slow client: %s", client.ConnID())
		}
	}
	return nil
}

func (m *ConnectionManager) subscribeToRoom(roomID, userID string) {
	subsI, _ := m.rooms.LoadOrStore(roomID, &RoomSubscribers{
		users:  make(map[string]bool),
		roomID: roomID,
	})
	subs := subsI.(*RoomSubscribers)
	subs.mu.Lock()
	subs.users[userID] = true
	subs.mu.Unlock()
}

func (m *ConnectionManager) unsubscribeFromRoom(roomID, userID string) {
	if subsI, ok := m.rooms.Load(roomID); ok {
		subs := subsI.(*RoomSubscribers)
		subs.mu.Lock()
		delete(subs.users, userID)
		subs.mu.Unlock()
	}
}
```
| | └── server.go:
```
package websocket

import (
	"net/http"

	"krampus/internal/domain"
	"krampus/pkg/config"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WebSocketServer struct {
	services      *service.Services
	config        *config.Config
	connectionMgr *ConnectionManager
}

func NewWebSocketServer(s *service.Services, cfg *config.Config) *WebSocketServer {
	return &WebSocketServer{
		services:      s,
		config:        cfg,
		connectionMgr: NewConnectionManager(),
	}
}

func (w *WebSocketServer) Start() {
	http.HandleFunc("/ws", w.handleWebSocket)
	println("WS server on /ws")
	http.ListenAndServe(w.config.HTTPPort, nil)
}

func (w *WebSocketServer) handleWebSocket(wr http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(wr, r, nil)
	if err != nil {
		println("Upgrade err:", err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("user_id")
	roomID := r.URL.Query().Get("room_id")
	token := r.URL.Query().Get("token")

	if userID == "" || roomID == "" || token == "" {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "missinng params"))
		return
	}

	if token != "valid" {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid token"))
		return
	}

	client := NewClient(conn, userID, roomID)
	w.connectionMgr.Register(client)
	defer w.connectionMgr.Unregister(client)

	println("WS connected:", userID, roomID)

	for {
		var msg domain.BaseMessage
		err := conn.ReadJSON(&msg)
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			println("WS read err:", err)
			break
		}

		msg.UserID = userID
		msg.RoomID = roomID

		w.services.MessageService.Process(r.Context(), &msg)
		w.connectionMgr.BroadcastToRoom(roomID, &msg)
	}
}
```
| ├── handler.go:
```
package adapters

import (
	"krampus/internal/domain"
	"krampus/internal/service"
	"krampus/pkg/config"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Router struct {
	services *service.Services
	config   config.Config
}

func NewRouter(s *service.Services, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ErrorMiddleware())
	r.Use(CORSMiddleware())
	r.Use(AuthMiddleware())

	api := r.Group("/v1")
	{
		api.POST("/messages", handleSendMessage(s))
		api.POST("/rooms", handleCreateRoom(s))
		api.GET("/rooms/:room_id/messages", handleGetMessages(s))
		api.GET("/rooms/:room_id", handleGetRoom(s))
		api.GET("/users/:user_id", handleGetUser(s))
	}

	r.GET("/health", handleHealth())
	r.GET("/metrics", handleMetrics())
	return r
}

func handleSendMessage(s *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Error(apperror.New(apperror.ErrUnauthorized, "no auth"))
			return
		}

		var msg domain.BaseMessage
		if err := c.ShouldBindJSON(&msg); err != nil {
			c.Error(apperror.New(apperror.ErrInvalidMessage, err.Error()))
			return
		}
		msg.UserID = userID.(string)

		if err := s.MessageService.Process(c.Request.Context(), &msg); err != nil {
			c.Error(err.(*apperror.AppError)) // уже AppError
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "sent", "msg_id": msg.ID})
	}
}

func handleGetMessages(s *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("room_id")
		limitStr := c.DefaultQuery("limit", "50")
		limit, _ := strconv.Atoi(limitStr)
		if limit < 1 || limit > 1000 {
			limit = 50
		}

		msgs, err := s.MessageService.GetRoomMessages(c.Request.Context(), roomID, limit)
		if err != nil {
			c.Error(err.(*apperror.AppError))
			return
		}
		c.JSON(http.StatusOK, gin.H{"messages": msgs})
	}
}

func handleCreateRoom(s *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id") // auth checked middleware

		var room domain.Room
		if err := c.ShouldBindJSON(&room); err != nil {
			c.Error(apperror.New(apperror.ErrInvalidMessage, err.Error()))
			return
		}
		room.OwnerID = userID.(string)
		ownerInMembers := false
		for _, m := range room.Members {
			if m == userID.(string) {
				ownerInMembers = true
				break
			}
		}
		if !ownerInMembers {
			room.Members = append(room.Members, room.OwnerID)
		}

		if err := s.RoomService.CreateRoom(c.Request.Context(), &room); err != nil {
			c.Error(err.(*apperror.AppError))
			return
		}
		c.JSON(http.StatusOK, gin.H{"room": room})
	}
}

func handleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(http.StatusOK, "metrics stub")
	}
}
```
| ├── middleware.go:
```
package adapters

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.Error(apperror.New(apperror.ErrUnauthorized, "no bearer"))
			c.Abort()
			return
		}

		userID := "user_" + auth[7:10]
		c.Set("user_id", userID)
		c.Next()
	}
}
```
| ├── routing.go:
```
package adapters

import (
	"github.com/gin-gonic/gin"
)

type UserHandler interface {
	SignUp(c *gin.Context)
	SignIn(c *gin.Context)
	Logout(c *gin.Context)
}

type RefreshTokenHandler interface {
	Refresh(c *gin.Context)
	// SendEmailToken(c *gin.Context)
}

type TwoFAHandler interface {
	VerifyCode(c *gin.Context)
	EnableTwoFA(c *gin.Context)
	// DisableTwoFA(c *gin.Context)
}

func RegisterRoutes(router *gin.RouterGroup, userHandler UserHandler, refreshTokenHandler RefreshTokenHandler, twoFAHandler TwoFAHandler) {
	auth := router.Group("/auth")
	{
		auth.POST("/register", userHandler.SignUp)
		auth.POST("/login", userHandler.SignIn)
		auth.POST("/logout", userHandler.Logout)

		auth.POST("/refresh", refreshTokenHandler.Refresh)
		// auth.POST("/send-code", refreshTokenHandler.SendEmailToken)

		auth.POST("/verify-code", twoFAHandler.VerifyCode)
		auth.POST("/enable-2fa", twoFAHandler.EnableTwoFA)
		// auth.POST("/disable-2fa", twoFAHandler.DisableTwoFA)
	}
}
```
| └── server.go:
```
package adapters

import (
	"context"
	"fmt"
	"krampus/internal/service"
	"krampus/pkg/config"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	httpServer *http.Server
	wsServer   *websocket.WebSocketServer
	// sseServer  *SSEServer stub
	// grpcServer *grpc.Server stub
	services *service.Services
	config   *config.Config
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

func New(s *service.Services, cfg *config.Config) *Server {
	return &Server{
		services: s,
		config:   cfg,
	}
}

func (s *Server) Start() {
	fmt.Println("Starting all servers...")
	s.wg.Add(3)

	go func() {
		defer s.wg.Done()
		s.startHTTP()
	}()

	go func() {
		defer s.wg.Done()
		s.wsServer = websocket.NewWebSocketServer(s.services, s.config)
		s.wsServer.Start()
	}()

	// go func() { defer s.wg.Done(); s.startSSE() }()

	// go func() { defer s.wg.Done(); s.startGRPC() }()

	s.wg.Wait()
}

func (s *Server) startHTTP() {
	r := NewRouter(s.services, s.config)
	s.httpServer = &http.Server{
		Addr:         s.config.HTTPPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	// println("HTTP on", s.config.HTTPPort)
	// 	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
	// 		panic(err)
	// 	}

	fmt.Printf("HTTP server starting on %s\n", s.config.HTTPPort)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func (s *Server) Stop(ctx context.Context) {
	// println("Shutting down...")
	// 	if s.httpServer != nil {
	// 		if err := s.httpServer.Shutdown(ctx); err != nil {
	// 			println("HTTP shutdown err:", err)
	// 		}
	// 	}
	// 	s.wg.Wait()
	// 	println("All stopped")
	fmt.Println("Graceful shutdown started...")

	if s.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP shutdown error: %v", err)
		}
	}

	if s.wsServer != nil {
		s.wsServer.Stop()
	}

	if s.sseServer != nil {
		s.sseServer.Stop()
	}

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	s.wg.Wait()
	fmt.Println("All servers stopped cleanly")
}
```
├── domain
| ├── chat-user.go:
```
package domain

type ChatUserStatus string

const (
	StatusOnline  ChatUserStatus = "online"
	StatusAway    ChatUserStatus = "away"
	StatusOffline ChatUserStatus = "offline"
	StatusDND     ChatUserStatus = "dnd"
)

type ChatUser struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Email       string                 `json:"email"`
	Status      ChatUserStatus         `json:"status"`
	LastActive  int64                  `json:"last_active"`
	CreatedAt   int64                  `json:"created_at"`
	Permissions []string               `json:"permissions"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type UserConnection struct {
	UserID      string `json:"user_id"`
	ConnID      string `json:"conn_id"`
	ClientInfo  string `json:"client_info"`
	IP          string `json:"ip"`
	ConnectedAt int64  `json:"connected_at"`
	Transport   string `json:"transport"`
}
```
| ├── message.go:
```
package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type MessageType string

const (
	TypeText        MessageType = "text"
	TypeFile        MessageType = "file"
	TypeCommand     MessageType = "command"
	TypeSystem      MessageType = "system"
	TypeVideoCall   MessageType = "video_call"
	TypeTyping      MessageType = "typing"
	TypeReadReceipt MessageType = "read_receipt"
)

type BaseMessage struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	UserID    string          `json:"user_id"`
	RoomID    string          `json:"room_id"`
	Timestamp int64           `json:"timestamp"`
	Version   int             `json:"verison"`
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
}

type TextPayload struct {
	Text      string              `json:"text"`
	Format    string              `json:"format"`
	Mentions  []string            `json:"mentions"`
	ReplyTo   string              `json:"reply_to"`
	Reactions map[string][]string `json:"reactions"`
}

type CommandPayload struct {
	Cmd     string                 `json:"cmd"`
	Args    []string               `json:"args"`
	Options map[string]interface{} `json:"options"`
}

type SystemPayload struct {
	Actions string      `json:"actions"`
	Data    interface{} `json:"data"`
}

type Metadata struct {
	Version     int               `json:"version"`
	Compression string            `json:"compression,omitempty"`
	Encryption  string            `json:"encryption,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

var ErrInvalidTimestamp = errors.New("invalid timestamp")

func (m *BaseMessage) Validate() error {
	if m.ID == "" || m.UserID == "" || m.RoomID == "" {
		return errors.New("missing required fields")
	}
	if m.Timestamp <= 0 {
		return ErrInvalidTimestamp
	}
	if m.Timestamp > time.Now().Add(5*time.Minute).UnixNano() {
		return ErrInvalidTimestamp
	}
	return nil
}

func (m *BaseMessage) SetTimestamp() {
	m.Timestamp = time.Now().UnixNano()
	m.ID = uuid.New().String()
}

func NewTextMessage(userID, roomID, text string) *BaseMessage {
	msg := &BaseMessage{
		Type:    TypeText,
		UserID:  userID,
		RoomID:  roomID,
		Version: 1,
	}
	payload, _ := json.Marshal(TextPayload{Text: text})
	msg.Payload = payload
	msg.SetTimestamp()
	return msg
}
```
| ├── room.go:
```
package domain

type RoomType string

const (
	RoomPersonal  RoomType = "personal"
	RoomPrivate   RoomType = "private"
	RoomGroup     RoomType = "group"
	RoomVideoCall RoomType = "video_call"
)

type Room struct {
	ID        string
	Type      RoomType
	OwnerID   string
	Name      string
	Members   []string
	CreatedAt int64
	UpdatedAt int64
	Settings  RoomSettings
}

type RoomMember struct {
	UserID     string
	RoomID     string
	Role       string
	JoinedAt   int64
	Permission []string
}

type RoomSettings struct {
	ReadOnly     bool
	ModerateMsgs bool
	AllowFiles   bool
	MaxMembers   int
	AutoArchive  bool
}
```
| ├── session.go:
```
package domain

import "time"

type CachedSession struct {
	UserID       int64     `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type CachedTempToken struct {
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
```
| └── user.go:
```
package domain

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Firstname    string    `json:"firstname"`
	Lastname     string    `json:"lastname"`
	Email        string    `json:"email"`
	Password     string    `json:"password"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	TwoFAEnabled bool      `json:"two_fa_enabled"`
	// BlockedUntil
	// FailedLogins
}

type TwoFaCodes struct {
	RequiresTwoFa bool   `json:"requires_two_fa"`
	TempToken     string `json:"temp_token"`
}

type TwoFaCode struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
	IsUsed    bool      `json:"is_used"`
	CreatedAt time.Time `json:"created_at"`
}

type Code struct {
	TempToken string `json:"temp_token"`
	Code      string `json:"code"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type TwoFaToggleRequest struct {
	Password string `json:"password"`
}
```
├── middleware
| └── auth.go:
```
package middleware

import (
	"krampus/internal/domain"
	"krampus/internal/storage/redis"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(redisStorage *redis.SessionStorage, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Извлекаем токен
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			c.Abort()
			return
		}

		// 2. ✅ ПРОВЕРЯЕМ РЕДИС КЭШ (100x быстрее JWT парсинга)
		cachedSession, err := redisStorage.GetAccessToken(tokenString)
		if err == nil {
			// ✅ ХИТ! Токен валиден в Redis
			c.Set("userID", cachedSession.UserID)
			c.Next()
			return
		}

		// 3. Cache MISS - парсим JWT
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user_id"})
			c.Abort()
			return
		}

		userID := int64(userIDFloat)

		// 4. ✅ КЭШИРУЕМ ВАЛИДНЫЙ ТОКЕН
		session := domain.CachedSession{
			UserID:       userID,
			AccessToken:  tokenString,
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			ExpiresAt:    time.Now().Add(15 * time.Minute),
		}
		if err := redisStorage.SetAccessToken(tokenString, session); err != nil {
			// Логируем, но продолжаем
		}

		c.Set("userID", userID)
		c.Next()
	}
}
```
├── service
| ├── message
| | └── service.go:
```
package message

import (
	"context"
	"krampus/internal/domain"
	"krampus/internal/storage"
	"krampus/pkg/apperror"
	"log"
	"time"
)

type MessageService struct {
	storage       storage.MessageStorage
	distributor   storage.MessageDistributor
	roomSvc       *RoomService
	userClientSvc *UserClientService
	rateLimiter   *RateLimiter
}

func NewMessageService(storage domain.MessageStorage, dist domain.MessageDistributor, roomSvc *RoomService, userClientSvc *UserClientService) *MessageService {
	return &MessageService{
		storage:       storage,
		distributor:   dist,
		roomSvc:       roomSvc,
		userClientSvc: userClientSvc,
		rateLimiter:   NewRateLimiter(),
	}
}

func (ms *MessageService) Process(ctx context.Context, msg *domain.BaseMessage) error {
	if msg.Timestamp == 0 {
		msg.SetTimestamp()
	}
	if err := msg.Validate(); err != nil {
		return apperror.New(apperror.ErrInvalidMessage, err.Error())
	}

	if err := ms.rateLimiter.Check(ctx, msg.UserID, msg.Type); err != nil {
		return err
	}

	user, err := ms.userClientSvc.GetUser(ctx, msg.UserID)
	if err != nil {
		return apperror.New(apperror.ErrUserNotFound, "user client not found")
	}

	room, err := ms.roomSvc.GetRoom(ctx, msg.RoomID)
	if err != nil {
		return apperror.New(apperror.ErrRoomNotFound, "room not found")
	}
	if err := ms.roomSvc.CanSendMessage(ctx, room, user, msg.Type); err != nil {
		return apperror.New(apperror.ErrForbidden, "no permission to send")
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ms.storage.SaveMessage(ctx, msg); err != nil {
			log.Printf("Failed to save message %s: %v", msg.ID, err)
		}
	}()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := ms.distributor.Broadcast(ctx, msg); err != nil {
			log.Printf("Failed to broadcast message %s: %v", msg.ID, err)
		}
	}()

	go ms.userClientSvc.UpdateLastActivity(ctx, msg.UserID)

	return nil
}

func (ms *MessageService) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error) {
	if _, err := ms.roomSvc.GetRoom(ctx, roomID); err != nill {
		return nil, apperror.New(apperror.ErrRoomNotFound, "room not found")
	}

	return ms.storage.GetRoomMessages(ctx, roomID, limit)
}
```
| ├── rateLimiter
| | └── service.go:
```
package rateLimiter

import (
	"context"
	"krampus/internal/domain"
	"krampus/pkg/apperror"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
	}
}

func (rl *RateLimiter) Check(ctx context.Context, userID string, msgType domain.MessageType) error {
	key := userID + ":" + string(msgType)

	rl.mu.Lock()
	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rl.createLimiter(msgType)
		rl.limiters[key] = limiter
	}
	rl.mu.Unlock()

	if !limiter.Allow() {
		return apperror.New(apperror.ErrRateLimit, "rate exceeded")
	}

	return nil
}

func (rl *RateLimiter) createLimiter(msgType domain.MessageType) *rate.Limiter {
	switch msgType {
	case domain.TypeText:
		return rate.NewLimiter(rate.Limit(10), 10)
	case domain.TypeCommand:
		return rate.NewLimiter(rate.Limit(2), 2)
	case domain.TypeTyping:
		return rate.NewLimiter(rate.Limit(5), 5)
	case domain.TypeFile:
		return rate.NewLimiter(rate.Limit(1), 3)
	default:
		return rate.NewLimiter(rate.Limit(50), 50)
	}
}

func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for key := range rl.limiters {
		if time.Since(rl.lastUsed[key]) > 10*time.Minute {
			delete(rl.limiters, key)
		}
	}
}
```
| ├── refreshToken
| | └── service.go:
```
package refreshToken

import (
	"errors"
	"time"

	"krampus/internal/domain"

	"github.com/golang-jwt/jwt/v5"
)

type RefreshTokenStorage interface {
	RefreshStore(userID int64, token string, expiresAt time.Time) error
	RefreshGet(token string) (int64, error)
	RefreshDelete(token string) error
	RefreshDeleteByUserID(userID int64) error
}

type RefreshToken struct {
	refreshTokenStorage RefreshTokenStorage
	jwtSecret           string
}

func NewUser(refreshToken RefreshTokenStorage, jwt string) *RefreshToken {
	return &RefreshToken{
		refreshTokenStorage: refreshToken,
		jwtSecret:           jwt,
	}
}

func (s *RefreshToken) GenerateRefreshToken(id int64) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = id
	claims["exp"] = time.Now().Add(7 * 24 * time.Hour).Unix()

	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	err = s.refreshTokenStorage.RefreshStore(id, signed, expiresAt)
	return signed, err
}

func (s *RefreshToken) UserRefresh(refreshToken string) (domain.TokenResponse, error) {
	userID, err := s.refreshTokenStorage.RefreshGet(refreshToken)
	if err != nil {
		return domain.TokenResponse{}, errors.New("Invalid refresh token")
	}

	accessToken, err := s.GenerateAccessToken(userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	newRefreshToken, err := s.GenerateRefreshToken(userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	s.refreshTokenStorage.RefreshDelete(refreshToken)

	return domain.TokenResponse{AccessToken: accessToken, RefreshToken: newRefreshToken}, nil
}

func (s *RefreshToken) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *RefreshToken) DeleteRefreshTokensByUserID(userID int64) error {
	return s.refreshTokenStorage.RefreshDeleteByUserID(userID)
}
```
| ├── room
| | └── service.go:
```
package room

import (
	"context"
	"krampus/internal/domain"
	"krampus/internal/storage"
	"time"
)

type RoomService struct {
	storage       storage.RoomStorage
	cache         storage.RoomCache
	userClientSvc *UserClientService
}

func NewRoomService(s storage.RoomStorage, cache storage.RoomCache, UserClientSvc *UserClientService) *RoomService {
	return &RoomService{
		storage:       s,
		cache:         cache,
		userClientSvc: UserClientSvc,
	}
}

func (rs *RoomService) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	if room := rs.cache.GetRoom(ctx, id); room != nil {
		return room, nil
	}

	room, err := rs.storage.GetRoom(ctx, id)
	if err != nil {
		return nil, err
	}

	go rs.cache.SetRoom(ctx, id, room)
	return room, nil
}

func (rs *RoomService) CanSendMesssage(ctx context.Context, room *domain.Room, user *domain.User, msgType domain.MessageType) bool {
	if !rs.isRoomMember(room, user.ID) {
		return msgType == domain.TypeSystem
	}

	switch room.Type {
	case domain.RoomPersonal:
		return room.OwnerID == user.ID
	case domain.RoomPrivate:
		return true
	case domain.RoomGroup:
		return true
	case domain.RoomVideoCall:
		return rs.isCallActive(room)
	}

	if room.Settings.ReadOnly && msgType != domain.TypeSystem {
		return false
	}

	if !room.Settings.AllowFiles && msgType == domain.TypeFile {
		return false
	}

	return true
}

func (rs *RoomService) isRoomMember(room *domain.Room, userID string) bool {
	for _, memberID := range room.Members {
		if memberID == userID {
			return true
		}
	}
	return false
}

func (rs *RoomService) CreateRoom(ctx context.Context, room *domain.Room) error {
	if err := rs.validateRoom(room); err != nil {
		return err
	}

	now := time.Now().UnixNano()
	room.CreatedAt = now
	room.UpdatedAt = now

	if err := rs.storage.SaveRoom(ctx, room); err != nil {
		return err
	}

	go rs.cache.SetRoom(ctx, room.ID, room)
	return nil
}
```
| ├── twofa
| | └── service.go:
```
package twofa

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"krampus/internal/domain"
	"krampus/internal/service/refreshToken"
	"krampus/internal/storage/redis"

	"github.com/golang-jwt/jwt/v5"
)

type TwoFAStorage interface {
	RenovationTwoFAStatus(userID int64, enabled bool) error
	InsertTwoFaCode(userID int64, code string, expiresAt time.Time) error
	SelectTwoFaCodeByUserID(userID int64) (domain.TwoFaCode, error)
	RenovationTwoFaCodeAttempts(codeID int64, attempts int64) error
	MarkTwoFaCodeUsed(codeID int64) error
	SelectRecentCodeRequests(userID int64, since time.Time) (int64, error)
	SelectRecentVerificationAttempts(userID int64, since time.Time) (int64, error)
}

type TwoFA struct {
	twoFAStorage        TwoFAStorage
	refreshTokenService *refreshToken.RefreshToken
	redisStorage        *redis.SessionStorage
	jwtSecret           string
}

func NewTwoFA(twoFA TwoFAStorage, refreshToken *refreshToken.RefreshToken, redisStorage *redis.SessionStorage, jwt string) *TwoFA {
	return &TwoFA{
		twoFAStorage:        twoFA,
		refreshTokenService: refreshToken,
		redisStorage:        redisStorage,
		jwtSecret:           jwt,
	}
}

func (s *TwoFA) EnableTwoFA(userID int64) error {
	return s.twoFAStorage.RenovationTwoFAStatus(userID, true)
}

func (s *TwoFA) UsersSendEmailCode(tempToken string) error {
	var userID int64

	cachedTemp, err := s.redisStorage.GetTempToken(tempToken)
	if err == nil && time.Now().Before(cachedTemp.ExpiresAt) {
		userID = cachedTemp.UserID
	} else {
		userID, err = s.extractUserIDFromToken(tempToken)
		if err != nil {
			return errors.New("Invalid temp token")
		}

		tempTokenData := domain.CachedTempToken{
			UserID:    userID,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		s.redisStorage.SetTempToken(tempToken, tempTokenData)
	}

	fifteenMinutesAgo := time.Now().Add(-15 * time.Minute)
	recentRequests, err := s.twoFAStorage.SelectRecentCodeRequests(userID, fifteenMinutesAgo)
	if err != nil {
		return err
	}

	if recentRequests >= 3 {
		return errors.New("too many code requests, please try again later")
	}

	code, err := s.generateSixDigitCode()
	if err != nil {
		return errors.New("failed to generate code")
	}

	expiresAt := time.Now().Add(5 * time.Minute)
	err = s.twoFAStorage.InsertTwoFaCode(userID, code, expiresAt)
	if err != nil {
		return err
	}

	err = s.sendEmail(userID, code)
	if err != nil {
		return err
	}

	return nil
}

func (s *TwoFA) VerifyCode(code domain.Code) (domain.TokenResponse, error) {
	userID, err := s.extractUserIDFromToken(code.TempToken)
	if err != nil {
		return domain.TokenResponse{}, errors.New("invalid temp token")
	}

	tenMinuteAgo := time.Now().Add(-10 * time.Minute)
	recentAttempts, err := s.twoFAStorage.SelectRecentVerificationAttempts(userID, tenMinuteAgo)
	if err != nil {
		return domain.TokenResponse{}, err
	}

	if recentAttempts >= 5 {
		return domain.TokenResponse{}, errors.New("too many verification attempts, please try again later")
	}

	twoFaCode, err := s.twoFAStorage.SelectTwoFaCodeByUserID(userID)
	if err != nil {
		return domain.TokenResponse{}, errors.New("invalid temp token or code not found")
	}

	if twoFaCode.IsUsed {
		return domain.TokenResponse{}, errors.New("code already used")
	}

	if twoFaCode.Attempts >= 3 {
		return domain.TokenResponse{}, errors.New("too many attempts")
	}

	if time.Now().After(twoFaCode.ExpiresAt) {
		return domain.TokenResponse{}, errors.New("code expires")
	}

	if twoFaCode.Code != code.Code {
		err = s.twoFAStorage.RenovationTwoFaCodeAttempts(twoFaCode.ID, int64(twoFaCode.Attempts+1))
		if err != nil {
			return domain.TokenResponse{}, err
		}

		remainingAttempts := 3 - (twoFaCode.Attempts + 1)
		return domain.TokenResponse{}, fmt.Errorf("invalid code, %d attempts remaining", remainingAttempts)
	}

	err = s.twoFAStorage.MarkTwoFaCodeUsed(twoFaCode.ID)
	if err != nil {
		return domain.TokenResponse{}, err
	}

	accessToken, err := s.GenerateAccessToken(twoFaCode.UserID)
	if err != nil {
		return domain.TokenResponse{}, err
	}

	refreshToken, err := s.refreshTokenService.GenerateRefreshToken(twoFaCode.UserID)
	if err != nil {
		return domain.TokenResponse{}, err
	}

	return domain.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *TwoFA) generateSixDigitCode() (string, error) {
	max := big.NewInt(899999)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", nil
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func (s *TwoFA) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *TwoFA) extractUserIDFromToken(tokenString string) (int64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("Invalid token claims")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("Invalid user_id in token")
	}

	return int64(userIDFloat), nil
}

func (s *TwoFA) sendEmail(userID int64, code string) error {
	fmt.Printf("Sending email to user %d: Your code: %s (valid for 5 minutes)\n", userID, code)
	return nil
}
```
| ├── user
| | └── service.go:
```
package service

import (
	"errors"
	"fmt"
	"krampus/internal/domain"
	"krampus/internal/service/refreshToken"
	"krampus/internal/storage/redis"
	"math"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserStorage interface {
	InsertUser(user domain.User) (int64, error)
	SelectUserByEmail(email string) (domain.User, error)
	SelectUserByID(userID int64) (domain.User, error)
	BlockUser(email, blockedUntil string) error
	RedisSessionStorage() redis.SessionStorage
}

type LoginAttemptStorage interface {
	LogAttempt(email string, result bool, attemptTime time.Time) error
	GetFailedLogAttempts(email string, windowStart time.Time) (int64, error)
	UserBlocked(email string, windowStart time.Time) ([]map[string]interface{}, error)
}

type User struct {
	userStorage         UserStorage
	loginAttemptStorage LoginAttemptStorage
	refreshTokenService *refreshToken.RefreshToken
	redisStorage        *redis.SessionStorage
	jwtSecret           string
}

func NewUser(
	user UserStorage,
	loginAttempt LoginAttemptStorage,
	refreshToken *refreshToken.RefreshToken,
	redisStorage *redis.SessionStorage,
	jwt string,
) *User {
	return &User{
		userStorage:         user,
		loginAttemptStorage: loginAttempt,
		refreshTokenService: refreshToken,
		redisStorage:        redisStorage,
		jwtSecret:           jwt,
	}
}

func (s *User) UserRegister(user domain.User) (domain.User, error) {
	fmt.Printf("DEBUG SERVICE REGISTER: Starting registration for: %s\n", user.Email)

	if user.Username == "" || user.Firstname == "" || user.Lastname == "" || user.Email == "" {
		return domain.User{}, errors.New("Invalid input: all fields are required")
	}

	if user.Password == "" || len(user.Password) < 8 {
		return domain.User{}, errors.New("Invalid password input: password must br at least 8 characters")
	}

	hasLetters, _ := regexp.MatchString(`[a-zA-Zа-яА-Я]`, user.Password)
	hasDigits, _ := regexp.MatchString(`[0-9]`, user.Password)
	hasSpecial, _ := regexp.MatchString(`[^a-zA-Zа-яА-Я0-9\s]`, user.Password)

	if !hasLetters || !hasDigits || !hasSpecial {
		return domain.User{}, errors.New("Invalid password input: password must contain letters, digits and special characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, errors.New("Error hashing password")
	}

	userToSave := domain.User{
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: string(hash),
		TwoFAEnabled: user.TwoFAEnabled,
	}

	fmt.Printf("DEBUG SERVICE REGISTER: Calling storage.InsertUser\n")
	id, err := s.userStorage.InsertUser(userToSave)
	if err != nil {
		fmt.Printf("DEBUG SERVICE REGISTER: Storage error: %v\n", err)
		return domain.User{}, err
	}

	createdUser := domain.User{
		ID:           id,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFAEnabled,
		CreatedAt:    time.Now(),
	}

	fmt.Printf("DEBUG SERVICE REGISTER: SUCCESS - Created student with ID: %d\n", id)
	return createdUser, nil
}

func (s *User) UserLogin(user domain.User) (domain.TokenResponse, domain.TwoFaCodes, error) {
	fmt.Printf("DEBUG LOGIN: Attempting login for email: '%s'\n", user.Email)
	fmt.Printf("DEBUG LOGIN: Password provided: '%s'\n", user.Password)
	fmt.Printf("DEBUG LOGIN: TwoFA enabled: '%v'\n", user.TwoFAEnabled)

	if user.Email == "" || user.Password == "" {
		fmt.Printf("DEBUG LOGIN: Email or password empty\n")
		return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("email and password are required")
	}

	blocked, minutesLeft, err := s.IsUserBlocked(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error checking block status: %v\n", err)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, err
	}

	if blocked {
		fmt.Printf("DEBUG LOGIN: User is blocked for %d minutes\n", minutesLeft)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, fmt.Errorf("your account is blocked for %d minutes", minutesLeft)
	}

	fmt.Printf("DEBUG LOGIN: Searching user in database...\n")
	dbUser, err := s.userStorage.SelectUserByEmail(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Database error or user not found: %v\n", err)
		s.LogLoginAttempt(user.Email, false)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGIN: User found - ID: %d, Email: %s\n", dbUser.ID, dbUser.Email)
	fmt.Printf("DEBUG LOGIN: Stored password hash: %s\n", dbUser.PasswordHash)
	fmt.Printf("DEBUG LOGIN: Provided password: %s\n", user.Password)

	fmt.Printf("DEBUG LOGIN: Comparing passwords...\n")
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(user.Password))
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Password comparison failed: %v\n", err)
		s.LogLoginAttempt(user.Email, false)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGIN: Password correct!\n")

	attempts, err := s.GetFailedAttempts(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error getting failed attempts: %v\n", err)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, err
	}

	maxAttempts := int64(5)
	if attempts >= maxAttempts {
		fmt.Printf("DEBUG LOGIN: Too many failed attempts: %d\n", attempts)
		s.BlockUser(user.Email)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("too many failed attempts, account blocked")
	}

	// if dbUser.TwoFAEnabled {
	if dbUser.TwoFAEnabled != false {
		tempToken, err := s.GenerateTempToken(dbUser.ID)
		if err != nil {
			fmt.Printf("DEBUG LOGIN: Error generating temp token: %v\n", err)
			return domain.TokenResponse{}, domain.TwoFaCodes{}, err
		}
		return domain.TokenResponse{}, domain.TwoFaCodes{RequiresTwoFa: true, TempToken: tempToken}, nil
	}

	accessToken, err := s.GenerateAccessToken(dbUser.ID)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error generating access token: %v\n", err)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, err
	}

	refreshToken, err := s.refreshTokenService.GenerateRefreshToken(dbUser.ID)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error generating refresh token: %v\n", err)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, err
	}

	session := domain.CachedSession{
		UserID:       dbUser.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	if err = s.redisStorage.SetAccessToken(accessToken, session); err != nil {
		fmt.Printf("Cache set error: %v\n", err)
	}
	if err = s.redisStorage.SetSessionByUserID(dbUser.ID, session); err != nil {
		fmt.Printf("Session cache error: %v\n", err)
	}

	s.LogLoginAttempt(user.Email, true)
	fmt.Printf("DEBUG LOGIN: Login successful for user ID: %d\n", dbUser.ID)
	return domain.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}, domain.TwoFaCodes{}, nil
}

func (s *User) UserLogout(userID int64, password string) error {
	fmt.Printf("DEBUG LOGOUT: Searching user in database...\n")
	dbUser, err := s.userStorage.SelectUserByID(userID)
	if err != nil {
		fmt.Printf("DEBUG LOGOUT: Database error or user not found: %v\n", err)
		s.LogLoginAttempt(dbUser.Email, false)
		return errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGOUT: User found - ID: %d, Email: %s\n", dbUser.ID, dbUser.Email)
	fmt.Printf("DEBUG LOGOUT: Stored password hash: %s\n", dbUser.PasswordHash)
	fmt.Printf("DEBUG LOGOUT: Provided password: %s\n", dbUser.Password)

	fmt.Printf("DEBUG LOGOUT: Comparing passwords...\n")
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(dbUser.Password))
	if err != nil {
		fmt.Printf("DEBUG LOGOUT: Password comparison failed: %v\n", err)
		s.LogLoginAttempt(dbUser.Email, false)
		return errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGOUT: Password correct!\n")

	if err = s.redisStorage.DeleteSessionByUserID(userID); err != nil {
		fmt.Printf("Redis session delete error: %v\n", err)
	}

	err = s.refreshTokenService.DeleteRefreshTokensByUserID(userID)
	if err != nil {
		return err
	}

	fmt.Printf("DEBUG LOGOUT: User %d successful logged out\n", userID)
	return nil
}

func (s *User) BlockUser(email string) {
	now := time.Now()
	blockedUntil := now.Add(1 * time.Minute).Format(time.RFC3339)

	s.LogLoginAttempt(email, false)

	err := s.userStorage.BlockUser(email, blockedUntil)
	if err != nil {
		fmt.Printf("Ошибка блокировки: %v\n", err)
	}
}

func (s *User) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *User) GenerateTempToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(10 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *User) LogLoginAttempt(email string, result bool) {
	attemptTime := time.Now().UTC()

	err := s.loginAttemptStorage.LogAttempt(email, result, attemptTime)
	if err != nil {
		fmt.Printf("Ошибка логирования: %v\n", err)
	}
}

func (s *User) GetFailedAttempts(email string) (int64, error) {
	now := time.Now().UTC()
	windowStart := now.Add(-1 * time.Minute)

	count, err := s.loginAttemptStorage.GetFailedLogAttempts(email, windowStart)
	if err != nil {
		fmt.Printf("Ошибка подсчета попыток: %v\n", err)
		return int64(0), err
	}

	return int64(count), err
}

func (s *User) IsUserBlocked(email string) (bool, int64, error) {
	now := time.Now().UTC()
	windowStart := now

	result, err := s.loginAttemptStorage.UserBlocked(email, windowStart)
	if err != nil {
		fmt.Printf("Ошибка проверки блокировки: %v\n", err)
		return false, 0, err
	}

	if len(result) > 0 {
		blockedUntilStr, ok := result[0]["blocked_until"].(string)
		if !ok {
			return false, 0, errors.New("invalid format for blocked_until")
		}

		blockedUntil, err := time.Parse(time.RFC3339, blockedUntilStr)
		if err != nil {
			return false, 0, err
		}

		minutesLeft := math.Ceil(time.Until(blockedUntil).Minutes())
		if minutesLeft < 0 {
			minutesLeft = 0
		}

		return true, int64(minutesLeft), nil
	}

	return false, 0, nil
}
```
| ├── userClient
| | └── service.go:
```
package chatuser

import (
	"context"
	"fmt"
	"krampus/internal/domain"
	"krampus/internal/storage"
	"log"
	"time"
)

type UserClientService struct {
	storage storage.UserClientStorage
	cache   storage.UserClientCache
}

func NewUserClientService(s storage.UserClientStorage, c storage.UserClientCache) *UserClientService {
	return &UserClientService{
		storage: s,
		cache:   c,
	}
}

func (ucs *UserClientService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	if user := ucs.cache.GetUser(ctx, id); user != nil {
		return user, nil
	}

	user, err := ucs.storage.GetUser(ctx, id)
	if err != nil {
		return nil, apperror.New(apperror.ErrUserNotFound, "user not found")
	}

	go ucs.cache.SetUser(ctx, id, user)
	return user, nil
}

func (ucs *UserClientService) UpdateLastActivity(ctx context.Context, userID string) {
	now := time.Now().UnixNano()

	if err := ucs.storage.UpdateLastActivity(ctx, userID, now); err != nil {
		log.Printf("Failed to update activity for %s: %v", userID, err)
		return
	}

	if user, err := ucs.cache.GetUser(ctx, userID); err == nil {
		user.LastActive = now
		go ucs.cache.SetUser(ctx, userID, user)
	}
}

func (ucs *UserClientService) ValidateUserPermissions(ctx context.Context, userID string, required []string) error {
	user, err := ucs.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	for _, requiredPerm := range required {
		hasPerm := false
		for _, userPerm := range user.Permissions {
			if userPerm == requiredPerm {
				hasPerm = true
				break
			}
		}
		if !hasPerm {
			return apperror.New(apperror.ErrConnection, fmt.Sprintf("missing permission: %s", requiredPerm))
		}
	}
	return nil
}

func (ucs *UserClientService) GetUserStatus(userID string) domain.UserStatus {
	user, err := ucs.GetUser(context.Background(), userID)
	if err != nil {
		return domain.StatusOffline
	}

	inactiveDuration := time.Since(time.Unix(0, user.LastActive))

	switch {
	case inactiveDuration < 5*time.Minute:
		return domain.StatusOnline
	case inactiveDuration < 30*time.Minute:
		return domain.StatusAway
	default:
		return domain.StatusOffline
	}
}
```
| └── services.go:
```
package service

type Services struct {
	MessageService    *MessageService
	RoomService       *RoomService
	UserClientService *UserClientService
}

func NewServices(st *storage.Storages) *Services {
	userClientSvc := NewUserClientService(st.UserClientStorage, st.UserClientCache)
	roomSvc := NewRoomService(st.RoomStorage, st.RoomCache)
	msgScv := NewMessageService(st.MessageStorage, st.MessageDistributor, roomSvc, userClientSvc)

	return &Services{
		MessageService:    msgScv,
		RoomService:       roomSvc,
		UserClientService: userClientSvc,
	}
}
```
├── sqlc
| ├── db.go:
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.30.0

package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

type Queries struct {
	db DBTX
}

func (q *Queries) WithTx(tx pgx.Tx) *Queries {
	return &Queries{
		db: tx,
	}
}
```
| ├── models.go:
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.30.0

package database

import (
	"github.com/jackc/pgx/v5/pgtype"
)

type LoginAttempt struct {
	ID          int64              `json:"id"`
	Email       string             `json:"email"`
	Success     bool               `json:"success"`
	AttemptedAt pgtype.Timestamptz `json:"attempted_at"`
	IpAddress   pgtype.Text        `json:"ip_address"`
	UserAgent   pgtype.Text        `json:"user_agent"`
}

type RefreshToken struct {
	ID        int64              `json:"id"`
	UserID    int64              `json:"user_id"`
	Token     string             `json:"token"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

type TwoFaCode struct {
	ID        int64              `json:"id"`
	UserID    int64              `json:"user_id"`
	Code      string             `json:"code"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
	Attempts  pgtype.Int4        `json:"attempts"`
	IsUsed    pgtype.Bool        `json:"is_used"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

type User struct {
	ID                int64              `json:"id"`
	Username          string             `json:"username"`
	Firstname         string             `json:"firstname"`
	Lastname          string             `json:"lastname"`
	Email             string             `json:"email"`
	PasswordHash      string             `json:"password_hash"`
	TwoFaEnabled      pgtype.Bool        `json:"two_fa_enabled"`
	CreatedAt         pgtype.Timestamptz `json:"created_at"`
	UpdatedAt         pgtype.Timestamptz `json:"updated_at"`
	BlockedUntil      pgtype.Timestamptz `json:"blocked_until"`
	FailedAttempts    pgtype.Int4        `json:"failed_attempts"`
	LastFailedAttempt pgtype.Timestamptz `json:"last_failed_attempt"`
}
```
| ├── querier.go:
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.30.0

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type Querier interface {
	BlockUser(ctx context.Context, arg BlockUserParams) error
	CreateLoginAttempt(ctx context.Context, arg CreateLoginAttemptParams) error
	CreateRefreshToken(ctx context.Context, arg CreateRefreshTokenParams) error
	CreateTwoFaCode(ctx context.Context, arg CreateTwoFaCodeParams) (int64, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	DeleteExpiredRefreshTokens(ctx context.Context) error
	DeleteRefreshToken(ctx context.Context, token string) error
	GetBlockedStatus(ctx context.Context, email string) (pgtype.Timestamptz, error)
	GetFailedLogAttempts(ctx context.Context, arg GetFailedLogAttemptsParams) (int64, error)
	GetRecentCodeRequests(ctx context.Context, arg GetRecentCodeRequestsParams) (int64, error)
	GetRecentFailedAttempts(ctx context.Context, arg GetRecentFailedAttemptsParams) (int64, error)
	GetRecentVerificationAttempts(ctx context.Context, arg GetRecentVerificationAttemptsParams) (int64, error)
	GetRefreshToken(ctx context.Context, token string) (GetRefreshTokenRow, error)
	GetTwoFaCodeByUserID(ctx context.Context, userID int64) (TwoFaCode, error)
	GetUserByEmail(ctx context.Context, email string) (GetUserByEmailRow, error)
	GetUserByID(ctx context.Context, id int64) (GetUserByIDRow, error)
	MarkTwoFaCodeAsUsed(ctx context.Context, id int64) error
	RefreshDeleteByUserI(ctx context.Context, userID int64) error
	RefreshDeleteByUserID(ctx context.Context, userID int64) error
	ResetFailedAttempts(ctx context.Context, email string) error
	UpdatePasswordHash(ctx context.Context, arg UpdatePasswordHashParams) error
	UpdateTwoFAStatus(ctx context.Context, arg UpdateTwoFAStatusParams) error
	UpdateTwoFaCodeAttempts(ctx context.Context, arg UpdateTwoFaCodeAttemptsParams) error
}

var _ Querier = (*Queries)(nil)
```
| ├── two_fa.sql.go:
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.30.0
// source: two_fa.sql

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const createTwoFaCode = `-- name: CreateTwoFaCode :one
INSERT INTO two_fa_codes (
    user_id,
    code,
    expires_at
) VALUES (
    $1, $2, $3
) RETURNING id
`

type CreateTwoFaCodeParams struct {
	UserID    int64              `json:"user_id"`
	Code      string             `json:"code"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
}

func (q *Queries) CreateTwoFaCode(ctx context.Context, arg CreateTwoFaCodeParams) (int64, error) {
	row := q.db.QueryRow(ctx, createTwoFaCode, arg.UserID, arg.Code, arg.ExpiresAt)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const getRecentCodeRequests = `-- name: GetRecentCodeRequests :one
SELECT COUNT(*) as count
FROM two_fa_codes 
WHERE user_id = $1 
  AND created_at >= $2
`

type GetRecentCodeRequestsParams struct {
	UserID    int64              `json:"user_id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

func (q *Queries) GetRecentCodeRequests(ctx context.Context, arg GetRecentCodeRequestsParams) (int64, error) {
	row := q.db.QueryRow(ctx, getRecentCodeRequests, arg.UserID, arg.CreatedAt)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getRecentVerificationAttempts = `-- name: GetRecentVerificationAttempts :one
SELECT COUNT(*) as count
FROM two_fa_codes 
WHERE user_id = $1 
  AND created_at >= $2 
  AND attempts > 0
`

type GetRecentVerificationAttemptsParams struct {
	UserID    int64              `json:"user_id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

func (q *Queries) GetRecentVerificationAttempts(ctx context.Context, arg GetRecentVerificationAttemptsParams) (int64, error) {
	row := q.db.QueryRow(ctx, getRecentVerificationAttempts, arg.UserID, arg.CreatedAt)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getTwoFaCodeByUserID = `-- name: GetTwoFaCodeByUserID :one
SELECT 
    id,
    user_id,
    code,
    expires_at,
    attempts,
    is_used,
    created_at
FROM two_fa_codes 
WHERE user_id = $1 
  AND is_used = false 
  AND expires_at > CURRENT_TIMESTAMP
ORDER BY created_at DESC 
LIMIT 1
`

func (q *Queries) GetTwoFaCodeByUserID(ctx context.Context, userID int64) (TwoFaCode, error) {
	row := q.db.QueryRow(ctx, getTwoFaCodeByUserID, userID)
	var i TwoFaCode
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.Code,
		&i.ExpiresAt,
		&i.Attempts,
		&i.IsUsed,
		&i.CreatedAt,
	)
	return i, err
}

const markTwoFaCodeAsUsed = `-- name: MarkTwoFaCodeAsUsed :exec
UPDATE two_fa_codes 
SET is_used = true 
WHERE id = $1
`

func (q *Queries) MarkTwoFaCodeAsUsed(ctx context.Context, id int64) error {
	_, err := q.db.Exec(ctx, markTwoFaCodeAsUsed, id)
	return err
}

const updateTwoFaCodeAttempts = `-- name: UpdateTwoFaCodeAttempts :exec
UPDATE two_fa_codes 
SET attempts = $2 
WHERE id = $1
`

type UpdateTwoFaCodeAttemptsParams struct {
	ID       int64       `json:"id"`
	Attempts pgtype.Int4 `json:"attempts"`
}

func (q *Queries) UpdateTwoFaCodeAttempts(ctx context.Context, arg UpdateTwoFaCodeAttemptsParams) error {
	_, err := q.db.Exec(ctx, updateTwoFaCodeAttempts, arg.ID, arg.Attempts)
	return err
}
```
| └── users.sql.go:
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.30.0
// source: users.sql

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const blockUser = `-- name: BlockUser :exec
UPDATE users
SET 
    blocked_until = $2,
    failed_attempts = failed_attempts + 1,
    last_failed_attempt = CURRENT_TIMESTAMP
WHERE email = $1
`

type BlockUserParams struct {
	Email        string             `json:"email"`
	BlockedUntil pgtype.Timestamptz `json:"blocked_until"`
}

func (q *Queries) BlockUser(ctx context.Context, arg BlockUserParams) error {
	_, err := q.db.Exec(ctx, blockUser, arg.Email, arg.BlockedUntil)
	return err
}

const createLoginAttempt = `-- name: CreateLoginAttempt :exec
INSERT INTO login_attempts (email, success, attempted_at)
VALUES ($1, $2, $3)
`

type CreateLoginAttemptParams struct {
	Email       string             `json:"email"`
	Success     bool               `json:"success"`
	AttemptedAt pgtype.Timestamptz `json:"attempted_at"`
}

func (q *Queries) CreateLoginAttempt(ctx context.Context, arg CreateLoginAttemptParams) error {
	_, err := q.db.Exec(ctx, createLoginAttempt, arg.Email, arg.Success, arg.AttemptedAt)
	return err
}

const createRefreshToken = `-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token, expires_at)
VALUES ($1, $2, $3)
`

type CreateRefreshTokenParams struct {
	UserID    int64              `json:"user_id"`
	Token     string             `json:"token"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
}

func (q *Queries) CreateRefreshToken(ctx context.Context, arg CreateRefreshTokenParams) error {
	_, err := q.db.Exec(ctx, createRefreshToken, arg.UserID, arg.Token, arg.ExpiresAt)
	return err
}

const createUser = `-- name: CreateUser :one
INSERT INTO users (
    username,
    firstname,
    lastname,
    email,
    password_hash,
    two_fa_enabled
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING id, username, firstname, lastname, email, password_hash, two_fa_enabled, created_at, updated_at, blocked_until, failed_attempts, last_failed_attempt
`

type CreateUserParams struct {
	Username     string      `json:"username"`
	Firstname    string      `json:"firstname"`
	Lastname     string      `json:"lastname"`
	Email        string      `json:"email"`
	PasswordHash string      `json:"password_hash"`
	TwoFaEnabled pgtype.Bool `json:"two_fa_enabled"`
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRow(ctx, createUser,
		arg.Username,
		arg.Firstname,
		arg.Lastname,
		arg.Email,
		arg.PasswordHash,
		arg.TwoFaEnabled,
	)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Username,
		&i.Firstname,
		&i.Lastname,
		&i.Email,
		&i.PasswordHash,
		&i.TwoFaEnabled,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.BlockedUntil,
		&i.FailedAttempts,
		&i.LastFailedAttempt,
	)
	return i, err
}

const deleteExpiredRefreshTokens = `-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at <= CURRENT_TIMESTAMP
`

func (q *Queries) DeleteExpiredRefreshTokens(ctx context.Context) error {
	_, err := q.db.Exec(ctx, deleteExpiredRefreshTokens)
	return err
}

const deleteRefreshToken = `-- name: DeleteRefreshToken :exec
DELETE FROM refresh_tokens 
WHERE token = $1
`

func (q *Queries) DeleteRefreshToken(ctx context.Context, token string) error {
	_, err := q.db.Exec(ctx, deleteRefreshToken, token)
	return err
}

const getBlockedStatus = `-- name: GetBlockedStatus :one
SELECT blocked_until
FROM users
WHERE email = $1
`

func (q *Queries) GetBlockedStatus(ctx context.Context, email string) (pgtype.Timestamptz, error) {
	row := q.db.QueryRow(ctx, getBlockedStatus, email)
	var blocked_until pgtype.Timestamptz
	err := row.Scan(&blocked_until)
	return blocked_until, err
}

const getFailedLogAttempts = `-- name: GetFailedLogAttempts :one
SELECT COUNT(*) as count 
FROM login_attempts 
WHERE email = $1 
  AND success = false 
  AND attempted_at >= $2
`

type GetFailedLogAttemptsParams struct {
	Email       string             `json:"email"`
	AttemptedAt pgtype.Timestamptz `json:"attempted_at"`
}

func (q *Queries) GetFailedLogAttempts(ctx context.Context, arg GetFailedLogAttemptsParams) (int64, error) {
	row := q.db.QueryRow(ctx, getFailedLogAttempts, arg.Email, arg.AttemptedAt)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getRecentFailedAttempts = `-- name: GetRecentFailedAttempts :one
SELECT COUNT(*) as count
FROM login_attempts 
WHERE email = $1 
  AND success = false 
  AND attempted_at >= $2
`

type GetRecentFailedAttemptsParams struct {
	Email       string             `json:"email"`
	AttemptedAt pgtype.Timestamptz `json:"attempted_at"`
}

func (q *Queries) GetRecentFailedAttempts(ctx context.Context, arg GetRecentFailedAttemptsParams) (int64, error) {
	row := q.db.QueryRow(ctx, getRecentFailedAttempts, arg.Email, arg.AttemptedAt)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getRefreshToken = `-- name: GetRefreshToken :one
SELECT user_id, expires_at FROM refresh_tokens 
WHERE token = $1 
LIMIT 1
`

type GetRefreshTokenRow struct {
	UserID    int64              `json:"user_id"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
}

func (q *Queries) GetRefreshToken(ctx context.Context, token string) (GetRefreshTokenRow, error) {
	row := q.db.QueryRow(ctx, getRefreshToken, token)
	var i GetRefreshTokenRow
	err := row.Scan(&i.UserID, &i.ExpiresAt)
	return i, err
}

const getUserByEmail = `-- name: GetUserByEmail :one
SELECT 
    id,
    username,
    firstname,
    lastname,
    email,
    password_hash,
    two_fa_enabled,
    created_at,
    blocked_until,
    failed_attempts
FROM users 
WHERE email = $1 
LIMIT 1
`

type GetUserByEmailRow struct {
	ID             int64              `json:"id"`
	Username       string             `json:"username"`
	Firstname      string             `json:"firstname"`
	Lastname       string             `json:"lastname"`
	Email          string             `json:"email"`
	PasswordHash   string             `json:"password_hash"`
	TwoFaEnabled   pgtype.Bool        `json:"two_fa_enabled"`
	CreatedAt      pgtype.Timestamptz `json:"created_at"`
	BlockedUntil   pgtype.Timestamptz `json:"blocked_until"`
	FailedAttempts pgtype.Int4        `json:"failed_attempts"`
}

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (GetUserByEmailRow, error) {
	row := q.db.QueryRow(ctx, getUserByEmail, email)
	var i GetUserByEmailRow
	err := row.Scan(
		&i.ID,
		&i.Username,
		&i.Firstname,
		&i.Lastname,
		&i.Email,
		&i.PasswordHash,
		&i.TwoFaEnabled,
		&i.CreatedAt,
		&i.BlockedUntil,
		&i.FailedAttempts,
	)
	return i, err
}

const getUserByID = `-- name: GetUserByID :one
SELECT 
    id,
    username,
    firstname,
    lastname,
    email,
    password_hash,
    two_fa_enabled,
    created_at,
    blocked_until,
    failed_attempts
FROM users 
WHERE id = $1 
LIMIT 1
`

type GetUserByIDRow struct {
	ID             int64              `json:"id"`
	Username       string             `json:"username"`
	Firstname      string             `json:"firstname"`
	Lastname       string             `json:"lastname"`
	Email          string             `json:"email"`
	PasswordHash   string             `json:"password_hash"`
	TwoFaEnabled   pgtype.Bool        `json:"two_fa_enabled"`
	CreatedAt      pgtype.Timestamptz `json:"created_at"`
	BlockedUntil   pgtype.Timestamptz `json:"blocked_until"`
	FailedAttempts pgtype.Int4        `json:"failed_attempts"`
}

func (q *Queries) GetUserByID(ctx context.Context, id int64) (GetUserByIDRow, error) {
	row := q.db.QueryRow(ctx, getUserByID, id)
	var i GetUserByIDRow
	err := row.Scan(
		&i.ID,
		&i.Username,
		&i.Firstname,
		&i.Lastname,
		&i.Email,
		&i.PasswordHash,
		&i.TwoFaEnabled,
		&i.CreatedAt,
		&i.BlockedUntil,
		&i.FailedAttempts,
	)
	return i, err
}

const refreshDeleteByUserI = `-- name: RefreshDeleteByUserI :exec
DELETE FROM refresh_tokens 
WHERE user_id = $1
`

func (q *Queries) RefreshDeleteByUserI(ctx context.Context, userID int64) error {
	_, err := q.db.Exec(ctx, refreshDeleteByUserI, userID)
	return err
}

const refreshDeleteByUserID = `-- name: RefreshDeleteByUserID :exec
DELETE FROM refresh_tokens 
WHERE user_id = $1
`

func (q *Queries) RefreshDeleteByUserID(ctx context.Context, userID int64) error {
	_, err := q.db.Exec(ctx, refreshDeleteByUserID, userID)
	return err
}

const resetFailedAttempts = `-- name: ResetFailedAttempts :exec
UPDATE users 
SET 
    failed_attempts = 0,
    blocked_until = NULL 
WHERE email = $1
`

func (q *Queries) ResetFailedAttempts(ctx context.Context, email string) error {
	_, err := q.db.Exec(ctx, resetFailedAttempts, email)
	return err
}

const updatePasswordHash = `-- name: UpdatePasswordHash :exec
UPDATE users 
SET password_hash = $2
WHERE id = $1
`

type UpdatePasswordHashParams struct {
	ID           int64  `json:"id"`
	PasswordHash string `json:"password_hash"`
}

func (q *Queries) UpdatePasswordHash(ctx context.Context, arg UpdatePasswordHashParams) error {
	_, err := q.db.Exec(ctx, updatePasswordHash, arg.ID, arg.PasswordHash)
	return err
}

const updateTwoFAStatus = `-- name: UpdateTwoFAStatus :exec
UPDATE users
SET two_fa_enabled = $2
WHERE id = $1
`

type UpdateTwoFAStatusParams struct {
	ID           int64       `json:"id"`
	TwoFaEnabled pgtype.Bool `json:"two_fa_enabled"`
}

func (q *Queries) UpdateTwoFAStatus(ctx context.Context, arg UpdateTwoFAStatusParams) error {
	_, err := q.db.Exec(ctx, updateTwoFAStatus, arg.ID, arg.TwoFaEnabled)
	return err
}
```
├── storage
| ├── loginAttempt
| | └── repository.go:
```
package loginAttempt

import (
	"context"
	"time"

	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type StorageError struct {
	Message string
}

type Storage struct {
	queries *database.Queries
}

var (
	ErrTokenExpired = &StorageError{"token expired"}
)

func NewStorage(queries *database.Queries) *Storage {
	return &Storage{queries: queries}
}

func (e *StorageError) Error() string {
	return e.Message
}

func (s *Storage) LogAttempt(email string, result bool, attemptTime time.Time) error {
	params := database.CreateLoginAttemptParams{
		Email:       email,
		Success:     result,
		AttemptedAt: pgtype.Timestamptz{Time: attemptTime, Valid: true},
	}

	return s.queries.CreateLoginAttempt(context.Background(), params)
}

func (s *Storage) GetFailedLogAttempts(email string, windowStart time.Time) (int, error) {
	count, err := s.queries.GetRecentFailedAttempts(context.Background(), database.GetRecentFailedAttemptsParams{
		Email:       email,
		AttemptedAt: pgtype.Timestamptz{Time: windowStart, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

func (s *Storage) UserBlocked(email string, windowStart time.Time) ([]map[string]interface{}, error) {
	blockedUntil, err := s.queries.GetBlockedStatus(context.Background(), email)
	if err != nil {
		return []map[string]interface{}{}, nil
	}

	var isBlocked bool
	if blockedUntil.Valid && blockedUntil.Time.After(time.Now()) {
		isBlocked = true
	}

	return []map[string]interface{}{
		{
			"blocked_until": blockedUntil.Time, // Нет такого поля в структуре
			"is_blocked":    isBlocked,
		},
	}, nil
}
```
| ├── profile
| | └── repository.go
| ├── redis
| | └── session.go:
```
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"krampus/internal/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

type SessionStorage struct {
	client *redis.Client
}

func NewSessionStorage(client *redis.Client) *SessionStorage {
	return &SessionStorage{
		client: client,
	}
}

func (s *SessionStorage) SetAccessToken(token string, session domain.CachedSession) error {
	key := fmt.Sprintf("access: %s", token)

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	return s.client.Set(context.Background(), key, data, ttl).Err()
}

func (s *SessionStorage) GetAccessToken(token string) (domain.CachedSession, error) {
	key := fmt.Sprintf("access: %s", token)

	data, err := s.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return domain.CachedSession{}, fmt.Errorf("token not found")
	}
	if err != nil {
		return domain.CachedSession{}, fmt.Errorf("redis get: %w", err)
	}

	var session domain.CachedSession
	if err = json.Unmarshal([]byte(data), &session); err != nil {
		return domain.CachedSession{}, fmt.Errorf("json unmarshal: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		s.client.Del(context.Background(), key)
		return domain.CachedSession{}, fmt.Errorf("token expired")
	}

	return session, nil
}

func (s *SessionStorage) DeleteAccessToken(token string) error {
	key := fmt.Sprintf("access: %s", token)
	return s.client.Del(context.Background(), key).Err()
}

func (s *SessionStorage) GetSessionByUserID(userID int64) (domain.CachedTempToken, error) {
	key := fmt.Sprintf("session:%d", userID)
	data, err := s.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return domain.CachedTempToken{}, fmt.Errorf("temp token not found")
	}
	if err != nil {
		return domain.CachedTempToken{}, err
	}

	var tempToken domain.CachedTempToken
	json.Unmarshal([]byte(data), &tempToken)
	return tempToken, nil
}

func (s *SessionStorage) DeleteSessionByUserID(userID int64) error {
	key := fmt.Sprintf("session:%d", userID)
	return s.client.Del(context.Background(), key).Err()
}

func (s *SessionStorage) SetSessionByUserID(userID int64, session domain.CachedSession) error {
	key := fmt.Sprintf("session:%d", userID)
	data, _ := json.Marshal(session)
	return s.client.Set(context.Background(), key, data, 7*24*time.Hour).Err()
}

func (s *SessionStorage) SetTempToken(token string, temp domain.CachedTempToken) error {
	key := fmt.Sprintf("temp:%s", token)
	data, _ := json.Marshal(temp)
	ttl := time.Until(temp.ExpiresAt)
	return s.client.Set(context.Background(), key, data, ttl).Err()
}

func (s *SessionStorage) GetTempToken(token string) (domain.CachedTempToken, error) {
	key := fmt.Sprintf("temp:%s", token)
	data, err := s.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return domain.CachedTempToken{}, fmt.Errorf("temp token not found")
	}
	if err != nil {
		return domain.CachedTempToken{}, fmt.Errorf("redis get temp: %w", err)
	}

	var tempToken domain.CachedTempToken
	if err := json.Unmarshal([]byte(data), &tempToken); err != nil {
		return domain.CachedTempToken{}, fmt.Errorf("json unmarshal temp: %w", err)
	}

	return tempToken, nil
}
```
| ├── refreshToken
| | └── repository.go:
```
package refreshToken

import (
	"context"
	"time"

	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type StorageError struct {
	Message string
}

type Storage struct {
	queries *database.Queries
}

var (
	ErrTokenExpired = &StorageError{"token expired"}
)

func NewStorage(queries *database.Queries) *Storage {
	return &Storage{queries: queries}
}

func (e *StorageError) Error() string {
	return e.Message
}

func (s *Storage) RefreshStore(userID int64, token string, expiresAt time.Time) error {
	params := database.CreateRefreshTokenParams{
		UserID:    userID,
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}

	return s.queries.CreateRefreshToken(context.Background(), params)
}

func (s *Storage) RefreshGet(token string) (int64, error) {
	refreshToken, err := s.queries.GetRefreshToken(context.Background(), token)
	if err != nil {
		return 0, err
	}

	if time.Now().After(refreshToken.ExpiresAt.Time) {
		s.RefreshDelete(token)
		return 0, ErrTokenExpired
	}

	return refreshToken.UserID, nil
}

func (s *Storage) RefreshDelete(token string) error {
	return s.queries.DeleteRefreshToken(context.Background(), token)
}

func (s *Storage) RefreshDeleteByUserID(userID int64) error {
	return s.queries.RefreshDeleteByUserID(context.Background(), userID)
}
```
| ├── twofa
| | └── repository.go:
```
package twofa

import (
	"context"
	"time"

	"krampus/internal/domain"
	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type StorageError struct {
	Message string
}

type Storage struct {
	queries *database.Queries
}

var (
	ErrTokenExpired = &StorageError{"token expired"}
)

func NewStorage(queries *database.Queries) *Storage {
	return &Storage{queries: queries}
}

func (e *StorageError) Error() string {
	return e.Message
}

func (s *Storage) RenovationTwoFAStatus(userID int64, enabled bool) error {
	return s.queries.UpdateTwoFAStatus(context.Background(), database.UpdateTwoFAStatusParams{
		ID:           userID,
		TwoFaEnabled: pgtype.Bool{Bool: enabled, Valid: true},
	})
}

func (s *Storage) InsertTwoFaCode(userID int64, code string, expiresAt time.Time) error {
	_, err := s.queries.CreateTwoFaCode(context.Background(), database.CreateTwoFaCodeParams{
		UserID:    userID,
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	return err
}

func (s *Storage) SelectTwoFaCodeByUserID(userID int64) (domain.TwoFaCode, error) {
	code, err := s.queries.GetTwoFaCodeByUserID(context.Background(), userID)
	if err != nil {
		return domain.TwoFaCode{}, err
	}

	return domain.TwoFaCode{
		ID:        code.ID,
		UserID:    code.UserID,
		Code:      code.Code,
		ExpiresAt: code.ExpiresAt.Time,
		Attempts:  int(code.Attempts.Int32),
		IsUsed:    code.IsUsed.Bool,
		CreatedAt: code.CreatedAt.Time,
	}, nil
}

func (s *Storage) RenovationTwoFaCodeAttempts(codeID int64, attempts int) error {
	return s.queries.UpdateTwoFaCodeAttempts(context.Background(), database.UpdateTwoFaCodeAttemptsParams{
		ID:       codeID,
		Attempts: pgtype.Int4{Int32: int32(attempts), Valid: true},
	})
}

func (s *Storage) MarkTwoFaCodeUsed(codeID int64) error {
	return s.queries.MarkTwoFaCodeAsUsed(context.Background(), codeID)
}

func (s *Storage) SelectRecentCodeRequests(userID int64, since time.Time) (int, error) {
	count, err := s.queries.GetRecentCodeRequests(context.Background(), database.GetRecentCodeRequestsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

func (s *Storage) SelectRecentVerificationAttempts(userID int64, since time.Time) (int, error) {
	count, err := s.queries.GetRecentVerificationAttempts(context.Background(), database.GetRecentVerificationAttemptsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return int(count), nil
}
```
| ├── user
| | └── repository.go:
```
package twofa

import (
	"context"
	"time"

	"krampus/internal/domain"
	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type StorageError struct {
	Message string
}

type Storage struct {
	queries *database.Queries
}

var (
	ErrTokenExpired = &StorageError{"token expired"}
)

func NewStorage(queries *database.Queries) *Storage {
	return &Storage{queries: queries}
}

func (e *StorageError) Error() string {
	return e.Message
}

func (s *Storage) InsertUser(user domain.User) (int64, error) {
	params := database.CreateUserParams{
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFaEnabled: pgtype.Bool{Bool: user.TwoFAEnabled, Valid: true},
	}

	createdUser, err := s.queries.CreateUser(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return createdUser.ID, nil
}

func (s *Storage) SelectUserByEmail(email string) (domain.User, error) {
	user, err := s.queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return domain.User{}, err
	}

	return domain.User{
		ID:           user.ID,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFaEnabled.Bool,
		CreatedAt:    user.CreatedAt.Time,
	}, nil
}

func (s *Storage) SelectUserByID(id int64) (domain.User, error) {
	user, err := s.queries.GetUserByID(context.Background(), id)
	if err != nil {
		return domain.User{}, nil
	}

	return domain.User{
		ID:           user.ID,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFaEnabled.Bool,
		CreatedAt:    user.CreatedAt.Time,
	}, nil
}

func (s *Storage) BlockUser(email, blockedUntil string) error {
	var blockedUntilTime pgtype.Timestamptz
	if blockedUntil != "" {
		t, err := time.Parse(time.RFC3339, blockedUntil)
		if err != nil {
			return err
		}
		blockedUntilTime = pgtype.Timestamptz{Time: t, Valid: true}
	} else {
		blockedUntilTime = pgtype.Timestamptz{Valid: false}
	}

	return s.queries.BlockUser(context.Background(), database.BlockUserParams{
		Email:        email,
		BlockedUntil: blockedUntilTime,
	})
}
```
| ├── message
| | ├── file_storage.go:
```
// type FileStorage struct {
// 	basePath    string
// 	segmentSize time.Duration
// 	buffers     sync.Map // roomID -> *RoomBuffer
// }

// func New(basePath string, segmentSize time.Duration) *FileStorage {
// 	os.MkdirAll(basePath, 0755)
// 	return &FileStorage{basePath: basePath, segmentSize: segmentSize}
// }

// func (f *FileStorage) SaveMessage(msg *domain.BaseMessage) {
// 	bufferI, _ := f.buffers.LoadOrStore(msg.RoomID, &RoomBuffer{})
// 	buffer := bufferI.(*RoomBuffer)
// 	buffer.mu.Lock()
// 	defer buffer.mu.Unlock()

// 	buffer.messages = append(buffer.messages, msg)
// 	// size calc + flush if TypeSystem/Command or size>64MB or time>100ms
// 	if shouldFlush(buffer, msg) {
// 		f.flushBuffer(buffer)
// 	}
// }

type FileStorage struct {
	basePath    string
	segmentSize time.Duration
	buffers     sync.Map // roomID → *RoomBuffer
}

type RoomBuffer struct {
	mu         sync.Mutex
	messages   []*domain.BaseMessage
	size       int64
	lastFlush  time.Time
	activeFile *os.File
	writer     *bufio.Writer
}

func New(basePath string, segmentSize time.Duration) *FileStorage {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		log.Printf("Failed to create base directory: %v", err)
	}
	return &FileStorage{
		basePath:    basePath,
		segmentSize: segmentSize,
	}
}

func (f *FileStorage) SaveMessage(roomID string, msg *domain.BaseMessage) error {
	buffer := f.getOrCreateBuffer(roomID)
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	buffer.messages = append(buffer.messages, msg)
	messageSize := int64(len(msg.Payload) + 100) // + метаданные

	// 🔥 УМНАЯ СТРАТЕГИЯ FLUSH
	shouldFlush := false
	switch msg.Type {
	case domain.TypeSystem, domain.TypeCommand:
		shouldFlush = true // немедленная запись

	case domain.TypeText, domain.TypeFile:
		buffer.size += messageSize
		shouldFlush = buffer.size >= 64*1024 || time.Since(buffer.lastFlush) > 100*time.Millisecond

	case domain.TypeTyping, domain.TypeReadReceipt:
		shouldFlush = time.Since(buffer.lastFlush) > 500*time.Millisecond

	default:
		shouldFlush = len(buffer.messages) >= 50
	}

	if shouldFlush {
		return f.flushBuffer(roomID, buffer)
	}
	return nil
}

func (f *FileStorage) getOrCreateBuffer(roomID string) *RoomBuffer {
	actual, _ := f.buffers.LoadOrStore(roomID, &RoomBuffer{
		messages:  make([]*domain.BaseMessage, 0),
		lastFlush: time.Now(),
	})
	return actual.(*RoomBuffer)
}

func (f *FileStorage) flushBuffer(roomID string, buffer *RoomBuffer) error {
	if len(buffer.messages) == 0 {
		return nil
	}

	if err := f.ensureFile(roomID, buffer); err != nil {
		return err
	}

	// 📝 Запись всех сообщений
	for _, msg := range buffer.messages {
		line := f.formatMessageLine(msg)
		if _, err := buffer.writer.WriteString(line); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}

	if err := buffer.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	if err := buffer.activeFile.Sync(); err != nil {
		log.Printf("Failed to sync file: %v", err)
	}

	// 🧹 Очистка буфера
	buffer.messages = buffer.messages[:0]
	buffer.size = 0
	buffer.lastFlush = time.Now()

	return nil
}

func (f *FileStorage) ensureFile(roomID string, buffer *RoomBuffer) error {
	now := time.Now()
	filePath := f.getSegmentPath(roomID, now)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	buffer.activeFile = file
	buffer.writer = bufio.NewWriterSize(file, 64*1024) // 64KB буфер
	return nil
}

// 🗂️ УМНАЯ СЕГМЕНТАЦИЯ ПО ТИПАМ КОМНАТ
func (f *FileStorage) getSegmentPath(roomID string, t time.Time) string {
	roomType := f.getRoomType(roomID)

	switch roomType {
	case domain.RoomTypeVideoCall:
		// Видеозвонки: 1ч сегменты
		return filepath.Join(f.basePath, "video_calls", roomID,
			t.Format("2006-01-02"), fmt.Sprintf("%d.log", t.Hour()))

	case domain.RoomTypeGroup:
		// Групповые: 4ч сегменты
		hour := (t.Hour() / 4) * 4
		return filepath.Join(f.basePath, "groups", roomID,
			t.Format("2006-01"), fmt.Sprintf("%s_%02d.log", t.Format("2006-01-02"), hour))

	case domain.RoomTypePrivate:
		// Личные: 1 день + шардинг
		shard := roomID[:2]
		return filepath.Join(f.basePath, "private", shard, roomID,
			t.Format("2006-01-02")+".log")

	case domain.RoomTypePersonal:
		// Заметки: 1 месяц + шардинг
		shard := roomID[:2]
		return filepath.Join(f.basePath, "personal", shard, roomID,
			t.Format("2006-01")+".log")

	default:
		return filepath.Join(f.basePath, "default", roomID,
			t.Format("2006-01-02")+".log")
	}
}

func (f *FileStorage) formatMessageLine(msg *domain.BaseMessage) string {
	return fmt.Sprintf("%d|%s|%s|%s|%s|%s\n",
		msg.Timestamp, msg.ID, msg.Type, msg.UserID, msg.RoomID, string(msg.Payload))
}

// type FileStorage struct {
//   basePath    string
//   segmentSize time.Duration
//   buffers     sync.Map  // roomID → *RoomBuffer
// }

// type RoomBuffer struct {
//   mu         sync.Mutex
//   messages   []*domain.BaseMessage
//   size       int64
//   lastFlush  time.Time
//   activeFile *os.File
//   writer     *bufio.Writer
// }

// func New(basePath string, segmentSize time.Duration) *FileStorage {
//   if err := os.MkdirAll(basePath, 0755); err != nil {
//     log.Printf("Failed to create base directory: %v", err)
//   }
//   return &FileStorage{
//     basePath:    basePath,
//     segmentSize: segmentSize,
//   }
// }

// func (f *FileStorage) SaveMessage(roomID string, msg *domain.BaseMessage) error {
//   buffer := f.getOrCreateBuffer(roomID)
//   buffer.mu.Lock()
//   defer buffer.mu.Unlock()

//   buffer.messages = append(buffer.messages, msg)
//   messageSize := int64(len(msg.Payload) + 100) // + метаданные

//   // 🔥 УМНАЯ СТРАТЕГИЯ FLUSH
//   shouldFlush := false
//   switch msg.Type {
//   case domain.TypeSystem, domain.TypeCommand:
//     shouldFlush = true // немедленная запись

//   case domain.TypeText, domain.TypeFile:
//     buffer.size += messageSize
//     shouldFlush = buffer.size >= 64*1024 || time.Since(buffer.lastFlush) > 100*time.Millisecond

//   case domain.TypeTyping, domain.TypeReadReceipt:
//     shouldFlush = time.Since(buffer.lastFlush) > 500*time.Millisecond

//   default:
//     shouldFlush = len(buffer.messages) >= 50
//   }

//   if shouldFlush {
//     return f.flushBuffer(roomID, buffer)
//   }
//   return nil
// }

// func (f *FileStorage) getOrCreateBuffer(roomID string) *RoomBuffer {
//   actual, _ := f.buffers.LoadOrStore(roomID, &RoomBuffer{
//     messages:  make([]*domain.BaseMessage, 0),
//     lastFlush: time.Now(),
//   })
//   return actual.(*RoomBuffer)
// }

// func (f *FileStorage) flushBuffer(roomID string, buffer *RoomBuffer) error {
//   if len(buffer.messages) == 0 {
//     return nil
//   }

//   if err := f.ensureFile(roomID, buffer); err != nil {
//     return err
//   }

//   // 📝 Запись всех сообщений
//   for _, msg := range buffer.messages {
//     line := f.formatMessageLine(msg)
//     if _, err := buffer.writer.WriteString(line); err != nil {
//       return fmt.Errorf("failed to write message: %w", err)
//     }
//   }

//   if err := buffer.writer.Flush(); err != nil {
//     return fmt.Errorf("failed to flush buffer: %w", err)
//   }

//   if err := buffer.activeFile.Sync(); err != nil {
//     log.Printf("Failed to sync file: %v", err)
//   }

//   // 🧹 Очистка буфера
//   buffer.messages = buffer.messages[:0]
//   buffer.size = 0
//   buffer.lastFlush = time.Now()

//   return nil
// }

// func (f *FileStorage) ensureFile(roomID string, buffer *RoomBuffer) error {
//   now := time.Now()
//   filePath := f.getSegmentPath(roomID, now)

//   if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
//     return fmt.Errorf("failed to create directory: %w", err)
//   }

//   file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
//   if err != nil {
//     return fmt.Errorf("failed to open file: %w", err)
//   }

//   buffer.activeFile = file
//   buffer.writer = bufio.NewWriterSize(file, 64*1024) // 64KB буфер
//   return nil
// }

// // 🗂️ УМНАЯ СЕГМЕНТАЦИЯ ПО ТИПАМ КОМНАТ
// func (f *FileStorage) getSegmentPath(roomID string, t time.Time) string {
//   roomType := f.getRoomType(roomID)

//   switch roomType {
//   case domain.RoomTypeVideoCall:
//     // Видеозвонки: 1ч сегменты
//     return filepath.Join(f.basePath, "video_calls", roomID,
//       t.Format("2006-01-02"), fmt.Sprintf("%d.log", t.Hour()))

//   case domain.RoomTypeGroup:
//     // Групповые: 4ч сегменты
//     hour := (t.Hour() / 4) * 4
//     return filepath.Join(f.basePath, "groups", roomID,
//       t.Format("2006-01"), fmt.Sprintf("%s_%02d.log", t.Format("2006-01-02"), hour))

//   case domain.RoomTypePrivate:
//     // Личные: 1 день + шардинг
//     shard := roomID[:2]
//     return filepath.Join(f.basePath, "private", shard, roomID,
//       t.Format("2006-01-02")+".log")

//   case domain.RoomTypePersonal:
//     // Заметки: 1 месяц + шардинг
//     shard := roomID[:2]
//     return filepath.Join(f.basePath, "personal", shard, roomID,
//       t.Format("2006-01")+".log")

//   default:
//     return filepath.Join(f.basePath, "default", roomID,
//       t.Format("2006-01-02")+".log")
//   }
// }

// func (f *FileStorage) formatMessageLine(msg *domain.BaseMessage) string {
//   return fmt.Sprintf("%d|%s|%s|%s|%s|%s\n",
//     msg.Timestamp, msg.ID, msg.Type, msg.UserID, msg.RoomID, string(msg.Payload))
// }
```
| | ├── message.go:
```
package message

import (
	"context"
	"fmt"
	"krampus/internal/domain"
	"log"
	"time"
)

// SaveMessage — ОДНО сообщение
func (p *PostgresStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
	query := `
    INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days')
  `

	createdAt := time.Unix(0, msg.Timestamp)
	_, err := p.db.ExecContext(ctx, query,
		msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp,
		msg.Payload, msg.Signature, createdAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	return nil
}

// SaveMessageBatch — 1000+ msg/сек транзакцией
func (p *PostgresStorage) SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
    INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days')
  `)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, msg := range msgs {
		createdAt := time.Unix(0, msg.Timestamp)
		_, err := stmt.ExecContext(ctx,
			msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp,
			msg.Payload, msg.Signature, createdAt,
		)
		if err != nil {
			return fmt.Errorf("failed to execute batch insert: %w", err)
		}
	}

	return tx.Commit()
}

// GetRoomMessages — история чата (последние N)
func (p *PostgresStorage) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error) {
	query := `
    SELECT id, type, user_id, room_id, timestamp, payload, signature
    FROM messages
    WHERE room_id = $1
    ORDER BY timestamp DESC
    LIMIT $2
  `

	rows, err := p.db.QueryContext(ctx, query, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.BaseMessage
	for rows.Next() {
		msg := &domain.BaseMessage{}
		var createdAt time.Time
		if err := rows.Scan(&msg.ID, &msg.Type, &msg.UserID, &msg.RoomID,
			&msg.Timestamp, &msg.Payload, &msg.Signature, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		msg.Timestamp = createdAt.UnixNano() // Postgres → UnixNano
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	// 🔄 Реверс: старые→новые (для UI)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// CleanupOldMessages — cron каждые 24ч
func (p *PostgresStorage) CleanupOldMessages(ctx context.Context) error {
	query := `DELETE FROM messages WHERE expires_at < NOW()`
	result, err := p.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cleanup old messages: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Cleaned up %d old messages", rowsAffected)
	return nil
}

// // SaveMessage — ОДНО сообщение
// func (p *PostgresStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
//   query := `
//     INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
//     VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days')
//   `

//   createdAt := time.Unix(0, msg.Timestamp)
//   _, err := p.db.ExecContext(ctx, query,
//     msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp,
//     msg.Payload, msg.Signature, createdAt,
//   )

//   if err != nil {
//     return fmt.Errorf("failed to save message: %w", err)
//   }
//   return nil
// }

// // SaveMessageBatch — 1000+ msg/сек транзакцией
// func (p *PostgresStorage) SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error {
//   tx, err := p.db.BeginTx(ctx, nil)
//   if err != nil {
//     return fmt.Errorf("failed to begin transaction: %w", err)
//   }
//   defer tx.Rollback()

//   stmt, err := tx.PrepareContext(ctx, `
//     INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
//     VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days')
//   `)
//   if err != nil {
//     return fmt.Errorf("failed to prepare statement: %w", err)
//   }
//   defer stmt.Close()

//   for _, msg := range msgs {
//     createdAt := time.Unix(0, msg.Timestamp)
//     _, err := stmt.ExecContext(ctx,
//       msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp,
//       msg.Payload, msg.Signature, createdAt,
//     )
//     if err != nil {
//       return fmt.Errorf("failed to execute batch insert: %w", err)
//     }
//   }

//   return tx.Commit()
// }

// // GetRoomMessages — история чата (последние N)
// func (p *PostgresStorage) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error) {
//   query := `
//     SELECT id, type, user_id, room_id, timestamp, payload, signature
//     FROM messages
//     WHERE room_id = $1
//     ORDER BY timestamp DESC
//     LIMIT $2
//   `

//   rows, err := p.db.QueryContext(ctx, query, roomID, limit)
//   if err != nil {
//     return nil, fmt.Errorf("failed to query messages: %w", err)
//   }
//   defer rows.Close()

//   var messages []*domain.BaseMessage
//   for rows.Next() {
//     msg := &domain.BaseMessage{}
//     var createdAt time.Time
//     if err := rows.Scan(&msg.ID, &msg.Type, &msg.UserID, &msg.RoomID,
//                        &msg.Timestamp, &msg.Payload, &msg.Signature, &createdAt); err != nil {
//       return nil, fmt.Errorf("failed to scan message: %w", err)
//     }
//     msg.Timestamp = createdAt.UnixNano() // Postgres → UnixNano
//     messages = append(messages, msg)
//   }

//   if err := rows.Err(); err != nil {
//     return nil, fmt.Errorf("rows iteration error: %w", err)
//   }

//   // 🔄 Реверс: старые→новые (для UI)
//   for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
//     messages[i], messages[j] = messages[j], messages[i]
//   }

//   return messages, nil
// }

// // CleanupOldMessages — cron каждые 24ч
// func (p *PostgresStorage) CleanupOldMessages(ctx context.Context) error {
//   query := `DELETE FROM messages WHERE expires_at < NOW()`
//   result, err := p.db.ExecContext(ctx, query)
//   if err != nil {
//     return fmt.Errorf("failed to cleanup old messages: %w", err)
//   }

//   rowsAffected, _ := result.RowsAffected()
//   log.Printf("Cleaned up %d old messages", rowsAffected)
//   return nil
// }
```
| | ├── postgres.go:
```
package message

import (
	"database/sql"
	"fmt"
	"time"
)

type PostgresStorage struct {
	db  *sql.DB
	dsn string
}

func NewPostgres(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	p := &PostgresStorage{db: db, dsn: dsn}
	return p, p.createTables()
}

func (p *PostgresStorage) createTables() error {
	queries := []string{
		// 👥 users
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
      		name TEXT NOT NULL,
        	email TEXT UNIQUE,
         	status TEXT DEFAULT 'offline',
          	last_active BIGINT,
           	created_at BIGINT NOT NULL,
            permissions JSONB DEFAULT '[]',
            metadata JSONB
		)`,
		// 🏠 rooms
		`CREATE TABLE IF NOT EXISTS rooms (
	      id TEXT PRIMARY KEY,
	      type TEXT NOT NULL,
	      owner_id TEXT NOT NULL,
	      name TEXT,
	      members JSONB NOT NULL,
	      settings JSONB,
	      created_at BIGINT NOT NULL,
	      updated_at BIGINT NOT NULL
	    )`,
		// 📨 messages (TTL + JSONB)
		`CREATE TABLE IF NOT EXISTS messages (
	      id TEXT PRIMARY KEY,
	      type TEXT NOT NULL,
	      user_id TEXT NOT NULL,
	      room_id TEXT NOT NULL,
	      timestamp BIGINT NOT NULL,
	      payload JSONB NOT NULL,
	      signature TEXT,
	      created_at TIMESTAMP DEFAULT NOW(),
	      expires_at TIMESTAMP
	    )`,
	}

	// 🔥 ИНДЕКСЫ ДЛЯ ПРОИЗВОДИТЕЛЬНОСТИ
	indexes := []string{
		// Быстрый поиск сообщений по комнате+времени (DESC для истории)
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_room_timestamp
	     ON messages(room_id, timestamp DESC)`,

		// Поиск сообщений пользователя
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_user_timestamp
	     ON messages(user_id, timestamp DESC)`,

		// TTL очистка (автоматическая)
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_expires
	     ON messages(expires_at) WHERE expires_at < NOW()`,

		// Комнаты по владельцу
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_rooms_owner
	     ON rooms(owner_id)`,
	}

	for _, query := range append(queries, indexes...) {
		if _, err := p.db.Exec(query); err != nil {
			return fmt.Errorf("Failed to create table/index: %w", err)
		}
	}
	return nil
}

// func (p *PostgrtesStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
// 	_, err := p.db.ExecContext(ctx, `INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature) VALUES ($, $2, $3, $4, $5, $6, $7`, msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp, msg.Payload, msg.Signature)
// 	return err
// }

// func (p *PostgresStorage) createTables() {
// 	p.db.Exec(`
// 		CREATE TABLE IF NOT EXISTS messages (
// 			id TEXT PRIMARY KEY,
// 			type TEXT,
// 			user_id TEXT,
// 			room_id TEXT,
// 			timestamp BIGINT,
// 			payload JSONB,
// 			signature TEXT,
// 			created_at TIMESTAMP DEFAULT NOW()
// 		);
// 		CREATE INDEX IF NOT EXISTS idx_room_timestamp ON messages(room_id, timestamp DESC);
// 	`)
// 	// rooms, users tables аналогично
// }

// type PostgresStorage struct {
//   db  *sql.DB
//   dsn string
// }

// func NewPostgres(dsn string) (*PostgresStorage, error) {
//   db, err := sql.Open("postgres", dsn)
//   if err != nil {
//     return nil, fmt.Errorf("failed to open postgres: %w", err)
//   }

//   // 🛠️ Pool настройки
//   db.SetMaxOpenConns(100)
//   db.SetMaxIdleConns(10)
//   db.SetConnMaxLifetime(5 * time.Minute)

//   if err := db.Ping(); err != nil {
//     return nil, fmt.Errorf("failed to ping postgres: %w", err)
//   }

//   p := &PostgresStorage{db: db, dsn: dsn}
//   return p, p.createTables()
// }

// func (p *PostgresStorage) createTables() error {
// 	queries := []string{
// 		// 👥 users
// 		`CREATE TABLE IF NOT EXISTS users (
//       id TEXT PRIMARY KEY,
//       name TEXT NOT NULL,
//       email TEXT UNIQUE,
//       status TEXT DEFAULT 'offline',
//       last_active BIGINT,
//       created_at BIGINT NOT NULL,
//       permissions JSONB DEFAULT '[]',
//       metadata JSONB
//     )`,

// 		// 🏠 rooms
// 		`CREATE TABLE IF NOT EXISTS rooms (
//       id TEXT PRIMARY KEY,
//       type TEXT NOT NULL,
//       owner_id TEXT NOT NULL,
//       name TEXT,
//       members JSONB NOT NULL,
//       settings JSONB,
//       created_at BIGINT NOT NULL,
//       updated_at BIGINT NOT NULL
//     )`,

// 		// 📨 messages (TTL + JSONB)
// 		`CREATE TABLE IF NOT EXISTS messages (
//       id TEXT PRIMARY KEY,
//       type TEXT NOT NULL,
//       user_id TEXT NOT NULL,
//       room_id TEXT NOT NULL,
//       timestamp BIGINT NOT NULL,
//       payload JSONB NOT NULL,
//       signature TEXT,
//       created_at TIMESTAMP DEFAULT NOW(),
//       expires_at TIMESTAMP
//     )`,
// 	}

// 	// 🔥 ИНДЕКСЫ ДЛЯ ПРОИЗВОДИТЕЛЬНОСТИ
// 	indexes := []string{
// 		// Быстрый поиск сообщений по комнате+времени (DESC для истории)
// 		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_room_timestamp
//      ON messages(room_id, timestamp DESC)`,

// 		// Поиск сообщений пользователя
// 		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_user_timestamp
//      ON messages(user_id, timestamp DESC)`,

// 		// TTL очистка (автоматическая)
// 		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_expires
//      ON messages(expires_at) WHERE expires_at < NOW()`,

// 		// Комнаты по владельцу
// 		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_rooms_owner
//      ON rooms(owner_id)`,
// 	}

// 	for _, query := range append(queries, indexes...) {
// 		if _, err := p.db.Exec(query); err != nil {
// 			return fmt.Errorf("failed to create table/index: %w", err)
// 		}
// 	}
// 	return nil
// }
```
| | ├── redis.go:
```
// func (r *RedisStorage) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
// 	data, err := r.client.Get(ctx, "room:"+id).Bytes()
// 	if err == redis.Nil {
// 		return nil, domain.ErrNotFound
// 	}
// 	var room domain.Room
// 	return &room, json.Unmarshal(data, &room)
// }

// func (r *RedisStorage) SetRoom(ctx context.Context, id string, room *domain.Room) error {
// 	data, _ := json.Marshal(room)
// 	return r.client.Set(ctx, "room:"+id, data, 10*time.Minute).Err()
// }

type RedisStorage struct {
	client *redis.Client
}

func NewRedisStorage(addr, password string, db int) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{client: client}, nil
}

// CacheRecentMessage — LRU 100 последних (pipeline)
func (r *RedisStorage) CacheRecentMessage(ctx context.Context, roomID string, msg *domain.BaseMessage) error {
	key := "recent_messages:" + roomID
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	pipe := r.client.Pipeline()
	pipe.LPush(ctx, key, msgJSON)
	pipe.LTrim(ctx, key, 0, 99) // топ 100
	pipe.Expire(ctx, key, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	return err
}

// GetRecentMessages — быстрый кэш для новых клиентов
func (r *RedisStorage) GetRecentMessages(ctx context.Context, roomID string) ([]*domain.BaseMessage, error) {
	key := "recent_messages:" + roomID
	msgsJSON, err := r.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get cached messages: %w", err)
	}

	var messages []*domain.BaseMessage
	for _, msgJSON := range msgsJSON {
		var msg domain.BaseMessage
		if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
			continue // пропускаем битые
		}
		messages = append(messages, &msg)
	}
	return messages, nil
}

// 👥 User Connections (активные сессии)
func (r *RedisStorage) SetUserConnection(ctx context.Context, userID string, conn *domain.UserConnection) error {
	key := "user_connections:" + userID
	connJSON, err := json.Marshal(conn)
	if err != nil {
		return fmt.Errorf("failed to marshal connection: %w", err)
	}

	if err := r.client.HSet(ctx, key, conn.ConnID, connJSON).Err(); err != nil {
		return fmt.Errorf("failed to set user connection: %w", err)
	}
	return r.client.Expire(ctx, key, 30*time.Minute).Err()
}

func (r *RedisStorage) RemoveUserConnection(ctx context.Context, userID, connID string) error {
	key := "user_connections:" + userID
	return r.client.HDel(ctx, key, connID).Err()
}

// Room/User Cache
func (r *RedisStorage) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	key := "room:" + id
	roomJSON, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("room not found in cache")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room from cache: %w", err)
	}

	var room domain.Room
	if err := json.Unmarshal([]byte(roomJSON), &room); err != nil {
		return nil, fmt.Errorf("failed to unmarshal room: %w", err)
	}
	return &room, nil
}

func (r *RedisStorage) SetRoom(ctx context.Context, id string, room *domain.Room) error {
	key := "room:" + id
	roomJSON, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("failed to marshal room: %w", err)
	}
	return r.client.Set(ctx, key, roomJSON, 10*time.Minute).Err()
}

// type RedisStorage struct {
//   client *redis.Client
// }

// func NewRedisStorage(addr, password string, db int) (*RedisStorage, error) {
//   client := redis.NewClient(&redis.Options{
//     Addr:         addr,
//     Password:     password,
//     DB:           db,
//     PoolSize:     100,
//     MinIdleConns: 10,
//     DialTimeout:  5 * time.Second,
//     ReadTimeout:  3 * time.Second,
//     WriteTimeout: 3 * time.Second,
//   })

//   ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//   defer cancel()

//   if err := client.Ping(ctx).Err(); err != nil {
//     return nil, fmt.Errorf("failed to connect to Redis: %w", err)
//   }

//   return &RedisStorage{client: client}, nil
// }

// // CacheRecentMessage — LRU 100 последних (pipeline)
// func (r *RedisStorage) CacheRecentMessage(ctx context.Context, roomID string, msg *domain.BaseMessage) error {
//   key := "recent_messages:" + roomID
//   msgJSON, err := json.Marshal(msg)
//   if err != nil {
//     return fmt.Errorf("failed to marshal message: %w", err)
//   }

//   pipe := r.client.Pipeline()
//   pipe.LPush(ctx, key, msgJSON)
//   pipe.LTrim(ctx, key, 0, 99)  // топ 100
//   pipe.Expire(ctx, key, 24*time.Hour)

//   _, err = pipe.Exec(ctx)
//   return err
// }

// // GetRecentMessages — быстрый кэш для новых клиентов
// func (r *RedisStorage) GetRecentMessages(ctx context.Context, roomID string) ([]*domain.BaseMessage, error) {
//   key := "recent_messages:" + roomID
//   msgsJSON, err := r.client.LRange(ctx, key, 0, -1).Result()
//   if err != nil {
//     return nil, fmt.Errorf("failed to get cached messages: %w", err)
//   }

//   var messages []*domain.BaseMessage
//   for _, msgJSON := range msgsJSON {
//     var msg domain.BaseMessage
//     if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
//       continue  // пропускаем битые
//     }
//     messages = append(messages, &msg)
//   }
//   return messages, nil
// }

// // 👥 User Connections (активные сессии)
// func (r *RedisStorage) SetUserConnection(ctx context.Context, userID string, conn *domain.UserConnection) error {
//   key := "user_connections:" + userID
//   connJSON, err := json.Marshal(conn)
//   if err != nil {
//     return fmt.Errorf("failed to marshal connection: %w", err)
//   }

//   if err := r.client.HSet(ctx, key, conn.ConnID, connJSON).Err(); err != nil {
//     return fmt.Errorf("failed to set user connection: %w", err)
//   }
//   return r.client.Expire(ctx, key, 30*time.Minute).Err()
// }

// func (r *RedisStorage) RemoveUserConnection(ctx context.Context, userID, connID string) error {
//   key := "user_connections:" + userID
//   return r.client.HDel(ctx, key, connID).Err()
// }

// // Room/User Cache
// func (r *RedisStorage) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
//   key := "room:" + id
//   roomJSON, err := r.client.Get(ctx, key).Result()
//   if err == redis.Nil {
//     return nil, fmt.Errorf("room not found in cache")
//   }
//   if err != nil {
//     return nil, fmt.Errorf("failed to get room from cache: %w", err)
//   }

//   var room domain.Room
//   if err := json.Unmarshal([]byte(roomJSON), &room); err != nil {
//     return nil, fmt.Errorf("failed to unmarshal room: %w", err)
//   }
//   return &room, nil
// }

// func (r *RedisStorage) SetRoom(ctx context.Context, id string, room *domain.Room) error {
//   key := "room:" + id
//   roomJSON, err := json.Marshal(room)
//   if err != nil {
//     return fmt.Errorf("failed to marshal room: %w", err)
//   }
//   return r.client.Set(ctx, key, roomJSON, 10*time.Minute).Err()
// }
```
| | ├── repository.go
| | └── SCHEMAS.sql:
```
-- Сообщения (основная таблица)
CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  user_id TEXT NOT NULL,
  room_id TEXT NOT NULL,
  timestamp BIGINT NOT NULL,
  payload JSONB NOT NULL,
  signature TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  expires_at TIMESTAMP
);
CREATE INDEX CONCURRENTLY idx_messages_room_timestamp ON messages(room_id, timestamp DESC);
CREATE INDEX CONCURRENTLY idx_messages_user_timestamp ON messages(user_id, timestamp DESC);

-- Комнаты
CREATE TABLE IF NOT EXISTS rooms (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  owner_id TEXT NOT NULL,
  name TEXT,
  members JSONB NOT NULL,
  settings JSONB,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);

-- Пользователи
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT UNIQUE,
  status TEXT DEFAULT 'offline',
  last_active BIGINT,
  created_at BIGINT NOT NULL,
  permissions JSONB DEFAULT '[]',
  metadata JSONB
);
```
| ├── repositories
| | ├── filestorage
| | | └── fileStorage.go
| | ├── postgres
| | | ├── message.go
| | | └── postgres.go
| | ├── redis
| | | └── redis.go
| | ├── interfaces.go
| | └── storages.go
| ├── interfaces.go:
```
package storage

import (
	"context"
	"krampus/internal/domain"
)

type MessageStorage interface {
	SaveMessage(ctx context.Context, msg *domain.BaseMessage) error
	SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error
	GetRoomMessage(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error)
	GetMessage(ctx context.Context, id string) (*domain.BaseMessage, error)
}

type RoomStorage interface {
	SaveRoom(ctx context.Context, room *domain.Room) error
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	UpdateRoom(ctx context.Context, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
	ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error)
}

type UserClientStorage interface {
	SaveUserClient(ctx context.Context, user *domain.User) error
	GetUserClient(ctx context.Context, id string) (*domain.UserClient, error)
	UpdateUserClient(ctx context.Context, user *domain.User) error
	UpdateLastActivity(ctx context.Context, id string, ts int64) error
}

type RoomCache interface {
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	SetRoom(ctx context.Context, id string, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
}

type UserClientCache interface {
	GetUserClient(ctx context.Context, id string) (*domain.User, error)
	SetUserClient(ctx context.Context, id string, user *domain.User) error
	DeleteUserClient(ctx context.Context, id string) error
}

type MessageDistributor interface {
	Broadcast(ctx context.Context, msg *domain.BaseMessage) error
	BroadcastToRoom(ctx context.Context, msg *domain.BaseMessage, roomID string) error
	SendToUserClient(ctx context.Context, userID string, msg *domain.BaseMessage) error
}

// // 📨 MessageStorage — сообщения (Postgres + FileStorage)
// type MessageStorage interface {
//   SaveMessage(ctx context.Context, msg *domain.BaseMessage) error
//   SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error
//   GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error)
//   GetMessage(ctx context.Context, id string) (*domain.BaseMessage, error)
// }

// // 🏠 RoomStorage — комнаты (Postgres)
// type RoomStorage interface {
//   SaveRoom(ctx context.Context, room *domain.Room) error
//   GetRoom(ctx context.Context, id string) (*domain.Room, error)
//   UpdateRoom(ctx context.Context, room *domain.Room) error
//   DeleteRoom(ctx context.Context, id string) error
// }

// // 👤 UserStorage — пользователи (Postgres)
// type UserStorage interface {
//   SaveUser(ctx context.Context, user *domain.User) error
//   GetUser(ctx context.Context, id string) (*domain.User, error)
//   UpdateUser(ctx context.Context, user *domain.User) error
//   UpdateLastActivity(ctx context.Context, id string, timestamp int64) error
// }

// // 🌐 MessageDistributor — рассылка
// type MessageDistributor interface {
//   Broadcast(ctx context.Context, msg *domain.BaseMessage) error
//   BroadcastToRoom(ctx context.Context, roomID string, msg *domain.BaseMessage) error
//   SendToUser(ctx context.Context, userID string, msg *domain.BaseMessage) error
// }

// // ⚡ RoomCache — кэш комнат (Redis TTL 10min)
// type RoomCache interface {
//   GetRoom(ctx context.Context, id string) (*domain.Room, error)
//   SetRoom(ctx context.Context, id string, room *domain.Room) error
//   DeleteRoom(ctx context.Context, id string) error
// }

// // 👥 UserCache — кэш пользователей (Redis TTL 10min)
// type UserCache interface {
//   GetUser(ctx context.Context, id string) (*domain.User, error)
//   SetUser(ctx context.Context, id string, user *domain.User) error
//   DeleteUser(ctx context.Context, id string) error
// }
```
| └── storages.go:
```
package storage

import (
	"context"
	"fmt"
	"krampus/internal/storage"
	"krampus/internal/storage/filestorage"
	"krampus/internal/storage/psql"
	"krampus/internal/storage/redisstorage"
	"krampus/pkg/client-database/redis"
	"krampus/pkg/config"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

type Storages struct {
	MessageStorage    storage.MessageStorage
	RoomStorage       storage.RoomStorage
	UserClientStorage storage.UserClientStorage
	RoomCache         storage.RoomCache
	UserClientCache   storage.UserClientCache
	// Redis, File, Kafka fields
	redisCli    *redis.Client
	fileStor    *filestorage.FileStorage
	kafkaProd   *kafka.Producer
	distributor storage.MessageDistributor
}

func NewStorages(cfg *config.Config) (*Storages, error) {
	ctx := context.Background()

	// Postgres
	pg, err := psql.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	// redis
	// rdb := redis.NewClient(&redis.Options{
	// 	Addr:         cfg.Redis.Addr,
	// 	Password:     cfg.Redis.Password,
	// 	DB:           cfg.Redis.DB,
	// 	PoolSize:     100,
	// 	MinIdleConns: 10,
	// 	DialTimeout:  5 * time.Second,
	// })
	redisStorage, err := redisstorage.NewRedisStorage(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to Redis: %w", err)
	}
	// if err := rdb.Ping(ctx).Err(); err != nil {
	// 	return nil, err
	// }

	// FileStorage
	fs := filestorage.New(cfg.File.BasePath, cfg.File.SegmentSize)

	// Kafka Producer
	kafkaProd, err := kafka.NewProducer(&kafka.Config{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Kafka.Topics.Incoming,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka producer: %w", err)
	}

	// Distributor
	distributor := NewMessageDistributor(redisStorage, kafkaProd)

	return &Storages{
		MessageStorage:    pg.MessageStorage(),
		RoomStorage:       pg.RoomStorage(),
		UserClientStorage: pg.UserClientStorage(),
		RoomCache:         redisStorage.RoomCache(),
		UserClientCache:   redisStorage.UserClientCache(),
		redisCli:          redisStorage.Client(),
		fileStor:          fs,
		kafkaProd:         kafkaProd,
		distributor:       distributor,
	}, nil
}

func (st *Storages) Close() {
	if st.kafkaProd != nil {
		st.kafkaProd.Close()
	}
	if st.redisCli != nil {
		st.redisCli.Close()
	}
	log.Println("All storage connections closed")
}

// type Storages struct {
//   // 🗄️ ОСНОВНЫЕ ХРАНИЛИЩА (Postgres ACID)
//   MessageStorage storage.MessageStorage      // сообщения (Postgres+Files)
//   RoomStorage    storage.RoomStorage         // комнаты (Postgres)
//   UserStorage    storage.UserStorage         // пользователи (Postgres)

//   // ⚡ КЭШИ И ВРЕМЕННЫЕ ДАННЫЕ
//   RoomCache      storage.RoomCache           // Redis TTL 10min
//   UserCache      storage.UserCache           // Redis TTL 10min
//   redisCli       *redis.Client               // сырой Redis

//   // 💾 ДОЛГОВРЕМЕННОЕ ХРАНЕНИЕ
//   fileStor       *filestorage.FileStorage    // архивы сообщений

//   // 📨 ОЧЕРЕДИ И MESSAGING
//   kafkaProd      *kafka.Producer             // исходящие очереди
//   // kafkaCons      *kafka.Consumer          // входящие (опционально)

//   // 🌐 РАСПРЕДЕЛИТЕЛЬ СООБЩЕНИЙ
//   distributor    *storage.MessageDistributor // WS Broadcast + Kafka
// }

// func NewStorages(cfg *config.Config) (*Storages, error) {
// 	ctx := context.Background()

// 	// 1️⃣ POSTGRES (основное хранилище ACID)
// 	pg, err := psql.NewPostgres(cfg.PostgresDSN)
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to connect to PostgreSQL: %w", err)
// 	}

// 	// 2️⃣ REDIS (кэш + сессии)
// 	redisStorage, err := redisstorage.NewRedisStorage(
// 		cfg.Redis.Addr,
// 		cfg.Redis.Password,
// 		cfg.Redis.DB,
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to connect to Redis: %w", err)
// 	}

// 	// 3️⃣ FILE STORAGE (долговременный архив)
// 	fileStor := filestorage.New(cfg.File.BasePath, cfg.File.SegmentSize)

// 	// 4️⃣ KAFKA PRODUCER (асинхронная очередь)
// 	kafkaProd, err := kafka.NewProducer(&kafka.Config{
// 		Brokers: cfg.Kafka.Brokers,
// 		Topic:   cfg.Kafka.Topics.Incoming,
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to create Kafka producer: %w", err)
// 	}

// 	// 5️⃣ DISTRIBUTOR (рассылка WS + Kafka)
// 	distributor := NewMessageDistributor(redisStorage, kafkaProd)

// 	// 🎯 СОБИРАЕМ ВСЕ В Storages
// 	return &Storages{
// 		MessageStorage: pg.MessageStorage(), // Postgres+Files
// 		RoomStorage:    pg.RoomStorage(),
// 		UserStorage:    pg.UserStorage(),
// 		RoomCache:      redisStorage.RoomCache(),
// 		UserCache:      redisStorage.UserCache(),
// 		redisCli:       redisStorage.Client(),
// 		fileStor:       fileStor,
// 		kafkaProd:      kafkaProd,
// 		distributor:    distributor,
// 	}, nil
// }

// func (s *Storages) Close() {
// 	if s.kafkaProd != nil {
// 		s.kafkaProd.Close() // Kafka producer
// 	}
// 	if s.redisCli != nil {
// 		s.redisCli.Close() // Redis pool
// 	}
// 	// Postgres закрывается автоматически (database/sql)
// 	log.Println("All storage connections closed")
// }
```
├── sql
|    ├── queries
|    |     ├── two_fa.sql:
```
-- name: CreateTwoFaCode :one
INSERT INTO two_fa_codes (
    user_id,
    code,
    expires_at
) VALUES (
    $1, $2, $3
) RETURNING id;

-- name: GetTwoFaCodeByUserID :one
SELECT 
    id,
    user_id,
    code,
    expires_at,
    attempts,
    is_used,
    created_at
FROM two_fa_codes 
WHERE user_id = $1 
  AND is_used = false 
  AND expires_at > CURRENT_TIMESTAMP
ORDER BY created_at DESC 
LIMIT 1;

-- name: UpdateTwoFaCodeAttempts :exec
UPDATE two_fa_codes 
SET attempts = $2 
WHERE id = $1; 

-- name: MarkTwoFaCodeAsUsed :exec
UPDATE two_fa_codes 
SET is_used = true 
WHERE id = $1;

-- name: GetRecentCodeRequests :one
SELECT COUNT(*) as count
FROM two_fa_codes 
WHERE user_id = $1 
  AND created_at >= $2;

-- name: GetRecentVerificationAttempts :one
SELECT COUNT(*) as count
FROM two_fa_codes 
WHERE user_id = $1 
  AND created_at >= $2 
  AND attempts > 0;
```
|    |     └── users.sql:
```
-- name: CreateUser :one
INSERT INTO users (
    username,
    firstname,
    lastname,
    email,
    password_hash,
    two_fa_enabled
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetUserByEmail :one
SELECT 
    id,
    username,
    firstname,
    lastname,
    email,
    password_hash,
    two_fa_enabled,
    created_at,
    blocked_until,
    failed_attempts
FROM users 
WHERE email = $1 
LIMIT 1;

-- name: GetUserByID :one
SELECT 
    id,
    username,
    firstname,
    lastname,
    email,
    password_hash,
    two_fa_enabled,
    created_at,
    blocked_until,
    failed_attempts
FROM users 
WHERE id = $1 
LIMIT 1;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token, expires_at)
VALUES ($1, $2, $3);

-- name: GetRefreshToken :one
SELECT user_id, expires_at FROM refresh_tokens 
WHERE token = $1 
LIMIT 1;

-- name: DeleteRefreshToken :exec
DELETE FROM refresh_tokens 
WHERE token = $1;

-- name: RefreshDeleteByUserI :exec
DELETE FROM refresh_tokens 
WHERE user_id = $1;

-- name: CreateLoginAttempt :exec
INSERT INTO login_attempts (email, success, attempted_at)
VALUES ($1, $2, $3);

-- name: GetRecentFailedAttempts :one
SELECT COUNT(*) as count
FROM login_attempts 
WHERE email = $1 
  AND success = false 
  AND attempted_at >= $2;

-- name: GetBlockedStatus :one
SELECT blocked_until
FROM users
WHERE email = $1;

-- name: UpdateTwoFAStatus :exec
UPDATE users
SET two_fa_enabled = $2
WHERE id = $1;

-- name: GetFailedLogAttempts :one
SELECT COUNT(*) as count 
FROM login_attempts 
WHERE email = $1 
  AND success = false 
  AND attempted_at >= $2;
  
-- name: BlockUser :exec 
UPDATE users
SET 
    blocked_until = $2,
    failed_attempts = failed_attempts + 1,
    last_failed_attempt = CURRENT_TIMESTAMP
WHERE email = $1;  

-- name: ResetFailedAttempts :exec 
UPDATE users 
SET 
    failed_attempts = 0,
    blocked_until = NULL 
WHERE email = $1;

-- name: UpdatePasswordHash :exec 
UPDATE users 
SET password_hash = $2
WHERE id = $1;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at <= CURRENT_TIMESTAMP;

-- name: RefreshDeleteByUserID :exec 
DELETE FROM refresh_tokens 
WHERE user_id = $1;
```
|    └── schema
|          └── init.sql:
```
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    firstname VARCHAR(100) NOT NULL,
    lastname VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    two_fa_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    blocked_until TIMESTAMP WITH TIME ZONE,
    failed_attempts INTEGER DEFAULT 0,
    last_failed_attempt TIMESTAMP WITH TIME ZONE
);

CREATE TABLE refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(512) UNIQUE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE two_fa_codes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    attempts INTEGER DEFAULT 0,
    is_used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE login_attempts (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    success BOOLEAN NOT NULL,
    attempted_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ip_address VARCHAR(45),
    user_agent TEXT
);

-- Индексы для оптимизации
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
CREATE INDEX idx_two_fa_codes_user_id ON two_fa_codes(user_id);
CREATE INDEX idx_two_fa_codes_expires_at ON two_fa_codes(expires_at);
CREATE INDEX idx_login_attempts_email ON login_attempts(email);
CREATE INDEX idx_login_attempts_attempted_at ON login_attempts(attempted_at);

-- Индекс для поиска по email и времени
CREATE INDEX idx_login_attempts_email_time ON login_attempts(email, attempted_at);
```
├── pkg
|    ├── apperror
|    |     ├── error.go:
```
package apperror

import (
	"fmt"
)

type ErrorCode string

const (
	ErrInvalidMessage  ErrorCode = "INVALID_MESSAGE"
	ErrUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrRateLimit       ErrorCode = "RATE_LIMIT"
	ErrRoomNotFound    ErrorCode = "ROOm_NOT_FOUND"
	ErrUserNotFound    ErrorCode = "USER_NOT_FOUND"
	ErrForbidden       ErrorCode = "FORBIDDEN"
	ErrStorage         ErrorCode = "STORAGE_ERROR"
	ErrConnection      ErrorCode = "CONNECTION_ERROR"
	ErrValidation      ErrorCode = "VALIDATION_ERROR"
	ErrDuplicate       ErrorCode = "DUPLICATE_ERROR"
	ErrTimeout         ErrorCode = "TIMEOUT_ERROR"
	ErrPayloadTooLarge ErrorCode = "PAYLOAD_TOO_LARGE"
	ErrInternal        ErrorCode = "INTERNAL_ERROR"
)

type AppError struct {
	Code             ErrorCode `json:"code,omitempty"`
	Message          string    `json:"message,omitempty"`
	DeveloperMessage string    `json:"developer_message,omitempty"`
	Details          string    `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func New(code ErrorCode, msg string) *AppError {
	return &AppError{
		Code:    code,
		Message: msg,
	}
}

func NewWithDetails(code ErrorCode, msg, details string) *AppError {
	return &AppError{
		Code:    code,
		Message: msg,
		Details: details,
	}
}
```
|    |     └── middleware.go:
```
package apperror

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type appHandler func(w http.ResponseWriter, r *http.Request) error

func JWTMiddleware(jwtSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("DEBUG JWT MIDDLEWARE: Checking auth for %s %s\n", r.Method, r.URL.Path)
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			fmt.Printf("DEBUG JWT MIDDLEWARE: Missing token\n")
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		fmt.Printf("DEBUG JWT MIDDLEWARE: Token: %s...\n", tokenString[:10])
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Invalid claims", http.StatusUnauthorized)
			return
		}
		studentID := int64(claims["user_id"].(float64))
		ctx := context.WithValue(r.Context(), "studentID", studentID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		lastErr := c.Errors.Last()
		if appErr, ok := lastErr.Err.(*AppError); ok {
			c.JSON(getHTTPStatus(appErr.Code), gin.H{
				"error": gin.H{
					"code":              appErr.Code,
					"message":           appErr.Message,
					"developer_message": appErr.DeveloperMessage,
					"details":           appErr.Details,
				},
			})
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    ErrInternal,
				"message": "internal server error",
			},
		})
		c.Abort()
	}
}

func getHTTPStatus(code ErrorCode) int {
	switch code {
	case ErrUnauthorized:
		return http.StatusUnauthorized
	case ErrForbidden:
		return http.StatusForbidden
	case ErrRateLimit:
		return http.StatusTooManyRequests
	case ErrRoomNotFound, ErrUserNotFound:
		return http.StatusNotFound
	case ErrValidation, ErrInvalidMessage, ErrPayloadTooLarge:
		return http.StatusBadRequest
	case ErrDuplicate:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
```
|    ├── auth
|    |     └── jwt.go:
```
package auth

import (
	"errors"
	"time"
	
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID int64, secret string, duration time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
		},
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	
	if err != nil {
		return nil, err
	}
	
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	
	return nil, errors.New("invalid token")
}

func ExtractUserIDFromToken(tokenString, secret string) (int64, error) {
	claims, err := ParseToken(tokenString, secret)
	if err != nil {
		return 0, nil
	}
	return claims.UserID, nil
}
```
|    ├── client-database
|    |     ├── cassandra
|    |     ├── postgresql
|    |     |     └── postgres.go:
```
// pkg/client-database/postgresql/client.go
package postgresql

import (
	"context"
	"fmt"
	"log"
	"time"

	"krampus/pkg/config"
	repeatable "krampus/pkg/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Client interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewClient(ctx context.Context, maxAttempts int, sc config.Config) (pool *pgxpool.Pool, err error) {
	dsn := fmt.Sprintf("%s", sc.PostgresDSN)
	err = repeatable.DoWithTries(func() error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		pool, err = pgxpool.New(ctx, dsn)
		if err != nil {
			return err
		}

		return nil
	}, maxAttempts, 5*time.Second)

	if err != nil {
		log.Fatal("Error do with tries postgresql")
	}

	return pool, nil
}
```
|    |     └── redis
|    |           └── redis.go:
```
package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
}

type Client struct {
	rdb *redis.Client
}

func New(cfg Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) RDB() *redis.Client {
	return c.rdb
}

func (c *Client) Close() error {
	return c.rdb.Close()
}
```
|    ├── config
|    |     └── config.go:
```
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string
	HTTPPort    string `env:"HTTP_PORT" default:":8080"`
	GRPCPort    string
	SSEPort     string
	PostgresDSN string `env:"POSTGRES_DSN"`
	Redis       RedisConfig
	Kafka       KafkaConfig
	File        FileConfig
}

type RedisConfig struct {
	Addr     string `env:"REDIS_ADDR" default:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD"`
	DB       int    `env:"REDIS_DB" default:"0"`
}

type KafkaConfig struct {
	Brokers []string `env:"KAFKA_BROKERS" default:"localhost:9092"`
	Topics  KafkaTopics
}

type KafkaTopics struct {
	Incoming  string `env:"KAFKA_TOPIC_INCOMING" default:"messages.incoming"`
	Validated string
	Saved     string
	Broadcast string
}

type FileConfig struct {
	BasePath     string        `env:"FILE_BASE_PATH" default:"./storage"`
	SegmentSize  time.Duration `env:"FILE_SEGMENT_SIZE" default:"1h"`
	BufferSize   int
	FlushTimeout time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:         getEnv("ENV", "development"),
		HTTPPort:    getEnv("HTTP_PORT", ":8080"),
		GRPCPort:    getEnv("GRPC_PORT", ":9090"),
		SSEPort:     getEnv("SSE_PORT", ":8081"),
		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://user:pass@localhost/chatdb?sslmode=disable"),
		File: FileConfig{
			BasePath:     getEnv("FILE_BASE_PATH", "./storage"),
			SegmentSize:  parseDuration(getEnv("FILE_SEGMENT_SIZE", "1h")),
			BufferSize:   parseSize(getEnv("FILE_BUFFER_SIZE", "64MB")),
			FlushTimeout: parseDuration(getEnv("FILE_FLUSH_TIMEOUT", "100ms")),
		},
	}

	cfg.Redis.Addr = getEnv("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", "")
	cfg.Redis.DB, _ = strconv.Atoi(getEnv("REDIS_DB", "0"))

	cfg.Kafka.Brokers = getEnvAsSlice("KAFKA_BROKERS", []string{"localhost:9092"})
	cfg.Kafka.Topics.Incoming = getEnv("KAFKA_TOPICS_INCOMING", "incoming")
	cfg.Kafka.Topics.Validated = getEnv("KAFKA_TOPICS_VALIDATED", "validated")
	cfg.Kafka.Topics.Saved = getEnv("KAFKA_TOPICS_SAVED", "saved")
	cfg.Kafka.Topics.Broadcast = getEnv("KAFKA_TOPICS_BROADCAST", "broadcast")

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsSlice(key string, defaultVal []string) []string {
	val := getEnv(key, strings.Join(defaultVal, ","))
	return strings.Split(val, ",")
}

func parseDuration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	return d
}

func parseSize(s string) int {
	// Stub: 64*1024*1024
	return 67108864
}
```
|    ├── hash
|    |     └── hash.go:
```
package hash

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

func GenerateHash(password string) (string, error) {
	if password == "" || len(password) == 0 {
		return "", errors.New("password must be at least 8 characters")
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("error hashing password")
	}
	
	return string(hash), nil
}

func GenerateHashPasskey(passkey string) (string, error) {
	if passkey == "" || len(passkey) == 0 {
		return "", errors.New("passkey must be at least 4 characters")
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(passkey), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("error hashing passkey")
	}
	
	return string(hash), nil
}

func GenerateHashWordpasskey(wordpasskey string) (string, error) {
	if wordpasskey == "" || len(wordpasskey) == 0 {
		return "", errors.New("wordpasskey must be at least 6 characters")
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(wordpasskey), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("error hashing wordpasskey")
	}
	
	return string(hash), nil
}

func GenerateHashCloudPassword() {
	
}

func CompareHashAndPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
```
|    ├── logging
|    |     └── logging.go:
```
package logging

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	
	"github.com/sirupsen/logrus"
)

type writeHook struct {
	Writer []io.Writer
	LogLevels []logrus.Level
}

func (hook *writeHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	for _, w := range hook.Writer {
		w.Write([]byte(line))
	}
	return err
}

func (hook *writeHook) Levels() []logrus.Level {
	return hook.LogLevels
}

var e *logrus.Entry

type Logger struct {
	*logrus.Entry
}

func GetLogger() *Logger {
	return &Logger{e}
}

func (l *Logger) GetLoggerWithField(k string, v interface{}) *Logger {
	return &Logger{l.WithField(k, v)}
}

func Init() {
	l := logrus.New()
	l.SetReportCaller(true)
	
	// Изменяем только формат для Docker
	if os.Getenv("DOCKER_ENV") == "true" || os.Getenv("APP_ENV") == "prod" {
		// JSON формат для Docker/production
		l.Formatter = &logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
				filename := path.Base(frame.File)
				return frame.Function, fmt.Sprintf("%s:%d", filename, frame.Line)
			},
		}
	} else {
		// Текстовый формат для локальной разработки
		l.Formatter = &logrus.TextFormatter{
			CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
				filename := path.Base(frame.File)
				return fmt.Sprintf("%s()", frame.Function), fmt.Sprintf("%s:%d", filename, frame.Line)
			},
			DisableColors: false,
			FullTimestamp: true,
		}
	}
	
	// Пытаемся создать директорию logs, но не паникуем если не получается
	if err := os.MkdirAll("logs", 0755); err != nil {
		// Если не можем создать директорию, просто пишем в stdout
		l.SetOutput(os.Stdout)
		l.Warnf("Cannot create logs directory: %v. Logging to stdout only.", err)
	} else {
		// Открываем файл для логов
		allFile, err := os.OpenFile("logs/all.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if err != nil {
			// Если не можем открыть файл, пишем только в stdout
			l.SetOutput(os.Stdout)
			l.Warnf("Cannot open log file: %v. Logging to stdout only.", err)
		} else {
			// Устанавливаем хук для записи в файл и stdout
			l.SetOutput(io.Discard)
			l.AddHook(&writeHook{
				Writer: []io.Writer{allFile, os.Stdout},
				LogLevels: logrus.AllLevels,		
			})
		}
	}
	
	// Устанавливаем уровень логирования из переменной окружения
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	l.SetLevel(level)
	
	e = logrus.NewEntry(l)
}
```
|    ├── middleware
|    |     └── auth.go:
```
package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
			})
			return
		}
		
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		fmt.Printf("DEBUG JWT MIDDLEWARE: Token: %s...\n", tokenString[:min(10, len(tokenString))])
		
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			return
		}
		
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token claims",
			})
			return
		}
		
		userID := int64(claims["user_id"].(float64))
		c.Set("userID", userID)
		c.Set("token", tokenString)
		
		c.Next()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```
|    ├── server
|    |     └── gin.go:
```
package server

import (
	"fmt"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Server struct {
	engine *gin.Engine
	logger *logrus.Logger
	port   string
}

type Config struct {
	Port         string
	Mode         string
	CorsOrigins  []string
	CorsEnabled  bool
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func NewServer(cfg Config, logger *logrus.Logger) *Server {
	if cfg.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	
	engine := gin.New()
	engine.Use(
		gin.Recovery(),
		RequestLogger(logger),
	)
	
	if cfg.CorsEnabled {
		engine.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.CorsOrigins,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Auth orization"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}
	
	return &Server{
		engine: engine,
		logger: logger,
		port:   cfg.Port,
	}
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) Start() error {
	s.logger.Infof("Starting server on :%s", s.port)
	return s.engine.Run(":" + s.port)
}

func (s *Server) RegisterRoutes(registerFunc func(*gin.Engine)) {
	registerFunc(s.engine)
}

func RequestLogger(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		
		c.Next()
		
		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()
		
		if query != "" {
			path = path + "?" + query
		}
		
		entry := logger.WithFields(logrus.Fields{
			"status": status,
			"latency": latency,
			"client_ip": clientIP,
			"method": method,
			"path": path,
			"user_agent": c.Request.UserAgent(),
		})
		
		if len(errorMessage) > 0 {
			entry.Error(errorMessage)
		} else {
			msg := fmt.Sprintf("%s %s %d %s", method, path, status, latency)
			if status >= 500 {
				entry.Error(msg)
			} else if status >= 400 {
				entry.Warn(msg)
			} else {
				entry.Info(msg)
			}
		}
	}
}
```
|    ├── utils
|    |     ├── random.go:
```
package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

func GenerateSixDigitCode() (string, error) {
	max := big.NewInt(899999)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func GenerateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}

func GenerateRandomPasskey() (string, error) {
	max := big.NewInt(8999)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()+1000), nil
}

func GenerateRandomWordpasskey(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}

func GenerateRandomCloudPassword() {
	
}
```
|    |     └── repeatable.go:
```
package utils

import "time"

func DoWithTries(fn func() error, attemtps int, delay time.Duration) (err error) {
	for attemtps > 0 {
		if err = fn(); err != nil {
			time.Sleep(delay)
			attemtps--
			
			continue
		}
		
		return nil
	}
	
	return
}
```
|    ├── validation
|    |     └── validation.go:
```
package validation

import (
	"regexp"
)

func ValidatePassword(password string) error {
	if len(password) < 0 {
		return ErrPasswordTooShort
	}
	
	hasLetters, _ := regexp.MatchString(`[a-zA-Zа-яА-Я]`, password)
	hasDigits, _ := regexp.MatchString(`[0-9]`, password)
	hasSpecial, _ := regexp.MatchString(`[^a-zA-Zа-яА-Я0-9\s]`, password)
	
	if !hasLetters || !hasDigits || !hasSpecial {
		return ErrPasswordComplexity
	}
	
	return nil
}

func ValidateEmail(email string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}
	return nil
}

var (
	ErrPasswordTooShort   = &ValidationError{"password must be at least 8 characters"}
	ErrPasswordComplexity = &ValidationError{"password must contain letters, digits and special characters"}
	ErrInvalidEmail       = &ValidationError{"invalid email format"}
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
```
├── grafana-provisioning
|    ├── dashboards
|    |     └── dashboards.yaml:
```
apiVersion: 1

providers:
  - name: "krampus"
    orgId: 1
    folder: ""
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
```
|    └── datasources
|          └── datasources.yaml:
```
apiVersion: 1

datasources:
  - name: Mimir
    type: prometheus
    url: http://mimir:8080/prometheus
    access: proxy
    isDefault: true

  - name: Loki
    type: loki
    url: http://loki:3100
    access: proxy

  - name: Tempo
    type: tempo
    url: http://tempo:3200
    access: proxy

  - name: Elasticsearch
    type: elasticsearch
    url: http://elasticsearch:9200
    access: proxy
    database: krampus-logs-*
```
├── .env:
```
# Database
DB_HOST=db
DB_PORT=5432
DB_NAME=taskmanager
DB_USER=postgres
DB_PASSWORD=postgres

# Application
APP_ENV=local
APP_PORT=8080
JWT_SECRET=your-secret-key-here
LOG_LEVEL=debug

# Monitoring
GRAFANA_PASSWORD=krampus2026_prod
MAIL_PASSWORD=pglg txoz kwpo oods

# 🔧 Dev (локальная разработка)
# ENV=development
# HTTP_PORT=:8080
# POSTGRES_DSN=postgres://user:pass@localhost:5432/chatdb?sslmode=disable
# REDIS_ADDR=localhost:6379
# KAFKA_BROKERS=localhost:9092

# 🚀 Production (Kubernetes)
# ENV=production
# HTTP_PORT=:8080
# POSTGRES_DSN=postgres://app:secret@prod-cluster/chatdb?sslmode=require
# REDIS_ADDR=redismaster:6379
# REDIS_PASSWORD=supersecret
# KAFKA_BROKERS=kafka-1:9092,kafka-2:9092,kafka-3:9092
# FILE_BASE_PATH=/data/storage
```
├── .env.example:
```
# HTTP_PORT=8080
# GRPC_PORT=9090
# SSE_PORT=8081
# ENV=development
# POSTGRES_DSN=postgres://user:pass@localhost/chatdb?sslmode=disable
# REDIS_ADDR=localhost:6379
# REDIS_PASSWORD=
# REDIS_DB=0
# KAFKA_BROKERS=localhost:9092
# KAFKA_TOPICS_INCOMING=incoming
# FILE_BASE_PATH=./storage
# FILE_SEGMENT_SIZE=1h

# Database
DB_HOST=db
DB_PORT=5432
DB_NAME=taskmanager
DB_USER=postgres
DB_PASSWORD=postgres

# Application
APP_ENV=local
APP_PORT=8080
JWT_SECRET=your-secret-key-here
LOG_LEVEL=debug

NODE_ENV='development'

APPLICATION_PORT=4000
APPLICATION_URL='http://localhost:${APPLICATION_PORT}'
ALLOWED_ORIGIN='http://localhost:3000'

COOKIES_SECRET='secret'
SESSION_SECRET='secret'
SESSION_NAME='session'
SESSION_DOMAIN='localhost'
SESSION_MAX_AGE='30d'
SESSION_HTTP_ONLY=true
SESSION_SECURE=false
SESSION_FOLDER='sessions:'

POSTGRES_USER='root'
POSTGRES_PASSWORD='123456'
POSTGRES_HOST='localhost'
POSTGRES_PORT=5433
POSTGRES_DB='full-authorization'
POSTGRES_URI='postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}'

REDIS_USER='default'
REDIS_PASSWORD='pass123456'
REDIS_HOST='localhost'
REDIS_PORT=6379
REDIS_URI='redis://${REDIS_USER}:${REDIS_PASSWORD}@${REDIS_HOST}:${REDIS_PORT}'
```
├── .gitignore:
```# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=taskmanager

# MongoDB
MONGO_HOST=localhost
MONGO_PORT=27017
MONGO_USER=admin
MONGO_PASSWORD=password

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# JWT
JWT_SECRET=your-super-secret-jwt-key-change-in-production

# App
PORT=8080
NODE_ENV=development```
├── ansible.cfg:
```[defaults]
host_key_checking = False
inventory = ./inventory
interpreter_python = /usr/bin/python3```
├── config.yaml:
```storage_path: "./storage/storage.db"
http_server:
  address: "localhost:8082"
  timeout: 4s
  idle_timeout: 30s
id_debug: true
env: "local"
listen: 
  type: port
  bind_ip: 0.0.0.0
  port: 10000
storage:
  host: "db"
  port: "5432"
  database: "postgres"
  username: "postgres"
  password: "postgres"```
├── docker-compose.yml:
```version: "3.8"

services:
  postgres:
    image: postgres:17-alpine
    container_name: krampus_db_one
    environment:
      POSTGRES_DB: ${DB_NAME:-krampusdbone}
      POSTGRES_USER: ${DB_USER:-postgres}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-password}
    ports:
      - "5445:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./sql/schema/init.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test:
        [
          "CMD-SHELL",
          "pg_isready -U ${DB_USER:-postgres} -d ${DB_NAME:-krampusdbone}",
        ]
      timeout: 20s
      retries: 10
      interval: 10s
      start_period: 40s
    restart: unless-stopped
    networks:
      - krampus-net

  cassandra:
    image: cassandra:4.1
    container_name: krampus_db_two
    ports:
      - "9042:9042"
      - "7000:7000"
    environment:
      CASSANDRA_CLUSTER_NAME: KrampusCluster
      CASSANDRA_SEEDS: cassandra
    volumes:
      - cassandra_data:/var/lib/cassandra/data
    healthcheck:
      test: ["CMD", "nodetool", "status"]
      timeout: 20s
      retries: 10
      interval: 10s
      start_period: 40s
    networks:
      - krampus-net

  mongodb:
    image: mongo:7-jammy
    container_name: krampus_db_three
    environment:
      MONGO_INITDB_ROOT_USERNAME: ${MONGO_USER:-admin}
      MONGO_INITDB_ROOT_PASSWORD: ${MONGO_PASSWORD:-password}
    ports:
      - "27018:27017"
    volumes:
      - mongo_data:/data/db
    healthcheck:
      test:
        [
          "CMD-SHELL",
          'echo ''db.runCommand("ping").ok'' | mongosh --quiet || exit 1',
        ]
      timeout: 20s
      retries: 10
      interval: 10s
      start_period: 40s
    restart: unless-stopped
    networks:
      - krampus-net

  redis:
    image: redis:7-alpine
    container_name: krampus_cache
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      timeout: 20s
      retries: 10
      interval: 10s
      start_period: 40s
    restart: unless-stopped
    networks:
      - krampus-net

  zookeeper:
    image: confluentinc/cp-zookeeper:7.5.0
    container_name: krampus_zookeeper
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
    ports:
      - "22181:2181"
    volumes:
      - zookeeper_data:/var/lib/zookeeper/data
      - zookeeper_log:/var/lib/zookeeper/log
    networks:
      - krampus-net

  kafka:
    image: confluentinc/cp-kafka:7.5.0
    container_name: krampus_kafka
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
      - "29092:29092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:29092,PLAINTEXT_HOST://localhost:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    volumes:
      - kafka_data:/var/lib/kafka/data
    networks:
      - krampus-net

  backend:
    build: .
    container_name: krampus_app
    ports:
      - "8888:8080"
    depends_on:
      postgres:
        condition: service_healthy
      cassandra:
        condition: service_started
      mongodb:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
      DB_NAME: ${DB_NAME:-krampusdbone}
      DB_USER: ${DB_USER:-postgres}
      DB_PASSWORD: ${DB_PASSWORD:-password}
      REDIS_HOST: redis
      REDIS_PORT: 6379
      JWT_SECRET: my-secret-key
      APP_PORT: "8080"
      APP_ENV: local
      LOG_LEVEL: debug
      DOCKER_ENV: "true"
      MAIL_TRANSPORT: "SMTP"
      MAIL_HOST: "smtp.gmail.com"
      MAIL_PORT: "587"
      MAIL_USER: "noobikcfb@gmail.com"
      MAIL_PASSWORD: ${MAIL_PASSWORD}
      MAIL_FROM: '"Мой Блог <noobikcfb@gmail.com>!"'
      MAIL_FROM_NAME: "Мой Блог"
    volumes:
      - ./backend:/app
      - ./krampus_content:/var/lib/krampus/content
      - ./config.yaml:/config.yaml:ro
      - app_custom_logs:/app/logs
    restart: unless-stopped
    networks:
      - krampus-net
    command: go run cmd/app/main.go

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    environment:
      - API_URL: http://backend:8080
    volumes:
      - ./frontend:/app
    depends_on:
      backend:
        condition: service_started
    command: npm run dev
    networks:
      - krampus-net

  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.13.0
    container_name: krampus_elasticsearch
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false # dev, вкл в prod
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
    ports:
      - "9200:9200"
    volumes:
      - es_data:/usr/share/elasticsearch/data
    healthcheck:
      test:
        ["CMD-SHELL", "curl -f http://localhost:9200/_cluster/health || exit 1"]
    networks:
      - krampus-net

  logstash:
    image: docker.elastic.co/logstash/logstash:8.13.0
    container_name: krampus_logstash
    ports:
      - "50000:50000"
    volumes:
      - ./logstash.conf:/usr/share/logstash/pipeline/logstash.conf:ro
    depends_on:
      - elasticsearch
    networks:
      - krampus-net

  kibana:
    image: docker.elastic.co/kibana/kibana:8.13.0
    container_name: krampus_kibana
    ports:
      - "5601:5601"
    environment:
      ELASTICSEARCH_HOSTS: http://elasticsearch:9200
    depends_on:
      - elasticsearch
    networks:
      - krampus-net

  mimir:
    image: grafana/mimir:latest
    container_name: krampus_mimir
    ports:
      - "9009:9009"
      - "8080:8080"
    command:
      - "-config.file=/etc/mimir.yaml"
    volumes:
      - ./mimir.yaml:/etc/mimir.yaml:ro
      - mimir_data:/data
    restart: unless-stopped
    networks:
      - krampus-net

  loki:
    image: grafana/loki:latest
    container_name: krampus_loki
    ports:
      - "3100:3100"
    command: -config.file=/etc/loki/local-config.yaml
    volumes:
      - ./loki-config.yaml:/etc/loki/local-config.yaml:ro
      - loki_data:/loki
    restart: unless-stopped
    networks:
      - krampus-net

  tempo:
    image: grafana/tempo:latest
    container_name: krampus_tempo
    ports:
      - "3200:3200"
      - "4317:4317"
      - "4318:4318"
    command: ["-config.file=/etc/tempo.yaml"]
    volumes:
      - ./tempo.yaml:/etc/tempo.yaml:ro
      - tempo_data:/tmp/tempo
    restart: unless-stopped
    networks:
      - krampus-net

  grafana:
    image: grafana/grafana-enterprise:latest
    container_name: krampus_grafana
    ports:
      - "3001:3000"
    depends_on:
      - mimir
      - loki
      - tempo
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD:-prodpass123}
      - GF_AUTH_ANONYMOUS_ENABLED=false
      - GF_AUTH_DISABLE_LOGIN_FORM=false
      - GF_SERVER_ROOT_URL=http://localhost:3001
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana-provisioning:/etc/grafana/provisioning:ro
    restart: unless-stopped
    networks:
      - krampus-net

  promtail:
    image: grafana/promtail:latest
    container_name: krampus_promtail
    volumes:
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
      - ./promtail-config.yaml:/etc/promtail/config.yml:ro
    restart: unless-stopped
    networks:
      - krampus-net
    depends_on:
      - loki

volumes:
  postgres_data:
  cassandra_data:
  mongo_data:
  redis_data:
  app_custom_logs:
  zookeeper_data:
  zookeeper_log:
  kafka_data:
  es_data:
  mimir_data:
  loki_data:
  tempo_data:
  grafana_data:

networks:
  krampus-net:
    driver: bridge```
├── docker-compose.prod.yml
├── Dockerfile:
```FROM golang:1.24.6-alpine AS builder

RUN addgroup -S appgroup && adduser -S appuser -G appgroup \
    && apk add --no-cache ca-certificates tzdata upx

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server ./cmd/app

RUN upx --best --lzma /server

RUN mkdir -p /app/logs && chown appuser:appgroup /app/logs

RUN mkdir -p /tmp && chown appuser:appgroup /tmp

FROM alpine:latest

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

COPY --from=builder --chown=appuser:appgroup /server /server
COPY --from=builder /app/config.yaml /config.yaml

COPY --from=builder --chown=appuser:appgroup /app/sql/ /app/sql/

RUN mkdir -p /app/logs && chown appuser:appgroup /app/logs

USER appuser

WORKDIR /

EXPOSE 8080

CMD ["/server"]```
├── go.mod
├── go.sum
├── Info.md
├── inventory:
```[webservers]
localhost ansible_connection=local
80.250.189.177 ansible_user=ubuntu```
├── logstash.conf:
```input {
  tcp {
    port => 50000
    codec => json_lines
  }
}

filter {
  date {
    match => [ "timestamp", "ISO8601" ]
  }
}

output {
  elasticsearch {
    hosts => ["elasticsearch:9200"]
    index => "krampus-logs-%{+YYYY.MM.dd}"
  }
}```
├── loki-config.yaml:
```
auth_enabled: false

server:
  http_listen_port: 3100
  grpc_listen_port: 9096

common:
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

schema_config:
  configs:
    - from: 2020-10-24
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h
```
├── Makefile:
```.PHONY: build
build: 
	go build -v ./cmd/app

.DEFAULT_GOAL := build```
├── mimir.yaml:
```multitenancy_enabled: false
server:
  http_listen_port: 8080

blocks_storage:
  backend: local
  local:
    directory: /tmp/mimir/tsdb-blocks

ruler_storage:
  backend: local
  local:
    directory: /tmp/mimir/rules

compactor:
  data_dir: /tmp/mimir/compactor
  block_retention: 24h

limits:
  ingestion_rate_mb: 8
  ingestion_burst_size_mb: 12
  max_global_series_per_user: 500k```
├── playbook.yml:
```---
- name: Update system
  hosts: localhost
  become: yes
  tasks:
    - name: script
      ansible.builtin.debug:
        msg: "Приветствую вас твари! 😄"
    - name: Update apt cache
      apt:
        update_cache: yes
    - name: Upgrade packages
      apt:
         upgrade: dist
    - name: Install nginx
      apt:
        name: nginx
        state: present
        
    - name: Start and enable nginx service
      systemd:
        name: nginx
        state: started
        enabled: yes
    - name: Allow nginx port 80 through firewall
      ufw:
        rule: allow
        port: '80'
        proto: tcp
    - name: Test nginx response
      uri:
        url: http://localhost
        status_code: 200
      register: curl_result
    - name: Show curl result
      ansible.builtin.debug:
      msg: "Nginx готов! IP: $(curl -s ifconfig.me)"```
├── promtail-config.yaml:
```server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: containers
    docker_sd_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 15s
    relabel_configs:
      - source_labels: ["__meta_docker_container_name"]
        regex: "/(krampus_.*)"
        target_label: "container"
      - source_labels: ["__meta_docker_container_log_stream"]
        target_label: "stream"
      - source_labels: ["__meta_docker_container_id"]
        target_label: "docker_id"```
├── README.md
├── sqlc.yaml:
```version: "2"
sql:
  - schema: "sql/schema"           
    queries: "sql/queries"         
    engine: "postgresql"           
    gen:
      go:
        package: "database"        
        out: "internal/storage/psql/sqlc"   
        sql_package: "pgx/v5"      
        emit_json_tags: true       
        emit_prepared_queries: true 
        emit_interface: true       
        emit_exact_table_names: false
        emit_empty_slices: true```
└── tempo.yaml:
```server:
  http_listen_port: 3200

distributor:
  receivers:
    otlp:
      protocols:
        http:
        grpc:

storage:
  trace:
    backend: local
    local:
      path: /tmp/tempo/traces

compactor:
  compaction:
    block_retention: 168h
    compaction_window: 1h```
