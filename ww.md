cmd/app/main.go:
```
package main

import (
	"context"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	authRest "krampus/internal/auth/adapters"
	authService "krampus/internal/auth/service"
	authStorage "krampus/internal/auth/storage"
	chatAdapters "krampus/internal/chat/adapters"
	chatService "krampus/internal/chat/service"
	chatStorage "krampus/internal/chat/storage"
	messageAdapters "krampus/internal/message/adapters"
	messageService "krampus/internal/message/service"
	messageStorage "krampus/internal/message/storage"
	sqlc "krampus/internal/sqlc"
	userRest "krampus/internal/user/adapters"
	userService "krampus/internal/user/service"
	userStorage "krampus/internal/user/storage"
	"krampus/pkg/apperror"
	"krampus/pkg/client-database/postgresql"
	redisClient "krampus/pkg/client-database/redis"
	"krampus/pkg/config"
	"krampus/pkg/logging"
	"krampus/pkg/messaging/kafka"
	"krampus/pkg/server"
)

func main() {
	logging.Init()
	logger := logging.GetLogger()
	logger.Infoln("🚀 Starting Crampus Modular Monolith")

	myLog := logrus.New()

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("❌ Config load failed: %v", err)
	}

	pool, err := postgresql.NewClient(ctx, 15, *cfg)
	if err != nil {
		logger.Fatalf("❌ Postgres failed: %v", err)
	}
	defer pool.Close()
	queries := sqlc.New(pool)

	rdbWrapper, err := redisClient.New(redisClient.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: 10,
	})
	if err != nil {
		logger.Fatalf("Redis failed: %v", err)
	}

	// 3. Kafka (объединенная настройка)
	kafkaCfg := config.KafkaConfig{
		Brokers: cfg.Kafka.Brokers,
		Topics:  cfg.Kafka.Topics,
	}
	msgDistributor := messageStorage.NewMessageDistributor(kafkaCfg, *logger)
	kafkaConsumer, _ := kafka.NewConsumer(kafkaCfg, *logger)

	jwtSecret := getJWTSecret()

	// 4. USER MODULE
	laStorage := userStorage.NewLoginAttemptStorage(queries)
	userPG := userStorage.NewUserStorage(queries)
	userRedis := userStorage.NewRedisSessionStorage(rdbWrapper.RDB())
	refreshStorage := userStorage.NewRefreshTokenStorage(queries)
	twofaStorage := authStorage.NewStorage(queries)

	refreshSvc := userService.NewRefreshToken(refreshStorage, jwtSecret)
	userSvc := userService.NewUser(userPG, laStorage, refreshSvc, userRedis, jwtSecret)
	twoFASvc := authService.NewTwoFA(twofaStorage, refreshSvc, userRedis, jwtSecret)
	refreshHandler := userRest.NewRefreshTokenHandler(refreshSvc, logger)
	userHandler := userRest.NewUserHandler(userSvc, logger)
	twofaHandler := authRest.NewTwoFAHandler(twoFASvc, logger)

	// 5. CHAT MODULE
	userClientRedis := chatStorage.NewUserClientCache(rdbWrapper)
	userClientStorage := chatStorage.NewUserClientPGStorage(queries)
	userRoomRedis := chatStorage.NewRoomCache(rdbWrapper)
	userRoomStorage := chatStorage.NewRoomPGStorage(queries)

	userClientSvc := chatService.NewUserClientService(userClientStorage, userClientRedis)
	roomSvc := chatService.NewRoomService(userRoomStorage, userRoomRedis, userClientSvc)

	// 6. MESSAGE MODULE
	_ = messageStorage.NewFileStorage(cfg.File.BasePath, cfg.File.SegmentSize, roomSvc)
	// msgRedis, err := messageStorage.NewRedisStorage(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Fatalf("Failed to init message redis: %v", err)
	}
	msgPG := messageStorage.NewMessagePGStorage(pool, queries)
	msgSvc := messageService.NewMessageService(msgPG, msgDistributor, roomSvc, userClientSvc)

	// 7. HANDLERS
	chatRouter := chatAdapters.NewRouter(userRedis, roomSvc, msgSvc, userClientSvc, cfg)
	wsServer := messageAdapters.NewWebSocketServer(msgSvc, cfg, kafkaConsumer)

	// 8. SERVER
	engine := gin.New()
	engine.Use(gin.Recovery(), apperror.CORSMiddleware(), apperror.ErrorMiddleware())

	api := engine.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			userHandler.RegisterRoutes(auth)
			refreshHandler.RegisterRoutes(auth)
			twofaHandler.RegisterRoutes(auth)
		}

		// Регистрация путей чата с защитой
		chatGroup := api.Group("/chat")
		chatGroup.Use(apperror.AuthMiddleware(*userRedis, jwtSecret))
		chatRouter.RegisterRoutes(chatGroup)
	}

	// WebSocket путь
	engine.GET("/ws", func(c *gin.Context) {
		wsServer.HandleWebSocket(c.Writer, c.Request)
	})

	// Настройка и старт сервера
	serverCfg := server.Config{
		Port: cfg.HTTPPort,
		Mode: cfg.Env,
	}

	srv := server.NewServer(serverCfg, myLog)

	logger.Infof("Server starting on :%s", serverCfg.Port)
	if err := srv.Start(); err != nil {
		logger.Fatalf("Server failed: %v", err)
	}
}

func healthHandler(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok", "timestamp": time.Now().UTC()})
}

// Utils
func overrideConfigFromEnv(cfg *config.Config) {
	if env := os.Getenv("APP_ENV"); env != "" {
		cfg.Env = env
	}
	// Если в окружении есть полная строка подключения
	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		cfg.PostgresDSN = dsn
	}
}

func getJWTSecret() string {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret
	}
	return "my-super-secret-key-change-in-prod"
}

func getPort() string {
	if p := os.Getenv("APP_PORT"); p != "" {
		return p
	}
	return "8080"
}
```

internal/auth/adapters/twofaHandler.go
```
package adapters

import (
	"krampus/internal/user/domain"
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

func (h *twoFAHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/verify", h.VerifyCode)
	rg.POST("/enable", h.EnableTwoFA)
	// rg.POST("/disable", h.DisableTwoFA) // раскомментируйте, когда метод будет готов
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
	userID, exists := c.Get("user_id")
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

internal/auth/domain/twofa.go
```
package domain

import "time"

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
```

internal/auth/service/twofaService.go
```
package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"krampus/internal/auth/domain"
	user "krampus/internal/user/domain"
	userService "krampus/internal/user/service"
	userStorage "krampus/internal/user/storage"

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
	refreshTokenService *userService.RefreshToken
	redisStorage        *userStorage.RedisSessionStorage
	jwtSecret           string
}

func NewTwoFA(twoFA TwoFAStorage, refreshToken *userService.RefreshToken, redisStorage *userStorage.RedisSessionStorage, jwt string) *TwoFA {
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

		tempTokenData := user.CachedTempToken{
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

func (s *TwoFA) VerifyCode(code user.Code) (user.TokenResponse, error) {
	userID, err := s.extractUserIDFromToken(code.TempToken)
	if err != nil {
		return user.TokenResponse{}, errors.New("invalid temp token")
	}

	tenMinuteAgo := time.Now().Add(-10 * time.Minute)
	recentAttempts, err := s.twoFAStorage.SelectRecentVerificationAttempts(userID, tenMinuteAgo)
	if err != nil {
		return user.TokenResponse{}, err
	}

	if recentAttempts >= 5 {
		return user.TokenResponse{}, errors.New("too many verification attempts, please try again later")
	}

	twoFaCode, err := s.twoFAStorage.SelectTwoFaCodeByUserID(userID)
	if err != nil {
		return user.TokenResponse{}, errors.New("invalid temp token or code not found")
	}

	if twoFaCode.IsUsed {
		return user.TokenResponse{}, errors.New("code already used")
	}

	if twoFaCode.Attempts >= 3 {
		return user.TokenResponse{}, errors.New("too many attempts")
	}

	if time.Now().After(twoFaCode.ExpiresAt) {
		return user.TokenResponse{}, errors.New("code expires")
	}

	if twoFaCode.Code != code.Code {
		err = s.twoFAStorage.RenovationTwoFaCodeAttempts(twoFaCode.ID, int64(twoFaCode.Attempts+1))
		if err != nil {
			return user.TokenResponse{}, err
		}

		remainingAttempts := 3 - (twoFaCode.Attempts + 1)
		return user.TokenResponse{}, fmt.Errorf("invalid code, %d attempts remaining", remainingAttempts)
	}

	err = s.twoFAStorage.MarkTwoFaCodeUsed(twoFaCode.ID)
	if err != nil {
		return user.TokenResponse{}, err
	}

	accessToken, err := s.GenerateAccessToken(twoFaCode.UserID)
	if err != nil {
		return user.TokenResponse{}, err
	}

	refreshToken, err := s.refreshTokenService.GenerateRefreshToken(twoFaCode.UserID)
	if err != nil {
		return user.TokenResponse{}, err
	}

	return user.TokenResponse{
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

internal/auth/storage/psqlTwofaStorage.go
```
package storage

import (
	"context"
	"time"

	"krampus/internal/auth/domain"
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

func (s *Storage) RenovationTwoFaCodeAttempts(codeID int64, attempts int64) error {
	return s.queries.UpdateTwoFaCodeAttempts(context.Background(), database.UpdateTwoFaCodeAttemptsParams{
		ID:       codeID,
		Attempts: pgtype.Int4{Int32: int32(attempts), Valid: true},
	})
}

func (s *Storage) MarkTwoFaCodeUsed(codeID int64) error {
	return s.queries.MarkTwoFaCodeAsUsed(context.Background(), codeID)
}

func (s *Storage) SelectRecentCodeRequests(userID int64, since time.Time) (int64, error) {
	count, err := s.queries.GetRecentCodeRequests(context.Background(), database.GetRecentCodeRequestsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *Storage) SelectRecentVerificationAttempts(userID int64, since time.Time) (int64, error) {
	count, err := s.queries.GetRecentVerificationAttempts(context.Background(), database.GetRecentVerificationAttemptsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}
```

internal/chat/adapters/chatHandler.go
```
package adapters

import (
	"krampus/internal/chat/domain"
	"krampus/internal/chat/service"
	messageDomain "krampus/internal/message/domain"
	messageService "krampus/internal/message/service"
	redisStorage "krampus/internal/user/storage"
	"krampus/pkg/apperror"
	"krampus/pkg/config"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Router struct {
	RoomService       *service.RoomService
	UserClientService *service.UserClientService
	MessageService    *messageService.MessageService
	config            config.Config
}

func NewRouter(rs *redisStorage.RedisSessionStorage, roomSvc *service.RoomService, msgSvc *messageService.MessageService, userSvc *service.UserClientService, cfg *config.Config) *Router {
	return &Router{
		RoomService:       roomSvc,
		UserClientService: userSvc,
		MessageService:    msgSvc,
		config:            *cfg,
	}
}

func (r *Router) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/messages", handleSendMessage(r.MessageService))
	rg.POST("/rooms", handleCreateRoom(r.RoomService))
	rg.GET("/rooms/:room_id/messages", handleGetMessages(r.MessageService))
	rg.GET("/rooms/:room_id", handleGetRoom(r.RoomService))
	rg.GET("/users/:user_id", handleGetUser(r.UserClientService))
}

func handleSendMessage(s *messageService.MessageService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Error(apperror.New(apperror.ErrUnauthorized, "no auth"))
			return
		}

		var msg messageDomain.BaseMessage
		if err := c.ShouldBindJSON(&msg); err != nil {
			c.Error(apperror.New(apperror.ErrInvalidMessage, err.Error()))
			return
		}
		msg.UserID = userID.(string)

		if err := s.Process(c.Request.Context(), &msg); err != nil {
			c.Error(err.(*apperror.AppError)) // уже AppError
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "sent", "msg_id": msg.ID})
	}
}

func handleGetMessages(s *messageService.MessageService) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("room_id")
		limitStr := c.DefaultQuery("limit", "50")
		limit, _ := strconv.Atoi(limitStr)
		if limit < 1 || limit > 1000 {
			limit = 50
		}

		msgs, err := s.GetRoomMessages(c.Request.Context(), roomID, limit)
		if err != nil {
			c.Error(err.(*apperror.AppError))
			return
		}
		c.JSON(http.StatusOK, gin.H{"messages": msgs})
	}
}

func handleCreateRoom(s *service.RoomService) gin.HandlerFunc {
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

		if err := s.CreateRoom(c.Request.Context(), &room); err != nil {
			c.Error(err.(*apperror.AppError))
			return
		}
		c.JSON(http.StatusOK, gin.H{"room": room})
	}
}

func handleGetRoom(s *service.RoomService) gin.HandlerFunc {
	return func(c *gin.Context) {
		room, err := s.GetRoom(c.Request.Context(), c.Param("room_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusOK, room)
	}
}

func handleGetUser(s *service.UserClientService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := s.GetUser(c.Request.Context(), c.Param("user_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, user)
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

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
```

internal/chat/domain/chat.go
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

type UserConnection struct {
	UserID      string `json:"user_id"`
	ConnID      string `json:"conn_id"`
	ClientInfo  string `json:"client_info"`
	IP          string `json:"ip"`
	ConnectedAt int64  `json:"connected_at"`
	Transport   string `json:"transport"`
}
```

internal/chat/service/roomService.go
```
package service

import (
	"context"
	"fmt"
	"krampus/internal/chat/domain"
	"krampus/internal/chat/storage"
	messageDomain "krampus/internal/message/domain"
	userDomain "krampus/internal/user/domain"
	"time"
)

type RoomStorage interface {
	SaveRoom(ctx context.Context, room *domain.Room) error
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	// UpdateRoom(ctx context.Context, room *domain.Room) error
	// DeleteRoom(ctx context.Context, id string) error
	// ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error)
}

type RoomCache interface {
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	SetRoom(ctx context.Context, id string, room *domain.Room) error
	// DeleteRoom(ctx context.Context, id string) error
}

type RoomService struct {
	storage       *storage.RoomPGStorage
	cache         *storage.RoomCache
	userClientSvc *UserClientService
}

func NewRoomService(s *storage.RoomPGStorage, cache *storage.RoomCache, UserClientSvc *UserClientService) *RoomService {
	return &RoomService{
		storage:       s,
		cache:         cache,
		userClientSvc: UserClientSvc,
	}
}

func (rs *RoomService) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	room, err := rs.cache.GetRoom(ctx, id)
	if err == nil && room != nil {
		return room, nil
	}

	room, err = rs.storage.GetRoom(ctx, id)
	if err != nil {
		return nil, err
	}

	go rs.cache.SetRoom(ctx, id, room)
	return room, nil
}

func (rs *RoomService) CanSendMessage(ctx context.Context, room *domain.Room, user *userDomain.User, msgType messageDomain.MessageType) bool {
	userIDStr := fmt.Sprintf("%d", user.ID)

	if !rs.isRoomMember(room, userIDStr) {
		return msgType == messageDomain.TypeSystem
	}

	switch room.Type {
	case domain.RoomPersonal:
		return room.OwnerID == userIDStr
	case domain.RoomPrivate:
		return true
	case domain.RoomGroup:
		return true
	case domain.RoomVideoCall:
		return rs.isCallActive(room)
	}

	if room.Settings.ReadOnly && msgType != messageDomain.TypeSystem {
		return false
	}

	if !room.Settings.AllowFiles && msgType == messageDomain.TypeFile {
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

// Заглушка, написать логику проверки статуса звонка
func (rs *RoomService) isCallActive(room *domain.Room) bool {
	if room.Type != domain.RoomVideoCall {
		return false
	}

	return len(room.Members) >= 2
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

func (rs *RoomService) validateRoom(room *domain.Room) error {
	if room.ID == "" {
		return fmt.Errorf("room ID cannot be empty")
	}
	if room.OwnerID == "" {
		return fmt.Errorf("room must have an owner")
	}

	switch room.Type {
	case domain.RoomPersonal, domain.RoomPrivate, domain.RoomGroup, domain.RoomVideoCall:
	default:
		return fmt.Errorf("invalid room type: %s", room.Type)
	}

	if room.Type == domain.RoomGroup && len(room.Members) == 0 {
		room.Members = append(room.Members, room.OwnerID)
	}

	return nil
}

func (rs *RoomService) UpdateRoom(ctx context.Context, room *domain.Room) error {
	room.UpdatedAt = time.Now().UnixNano()
	if err := rs.storage.UpdateRoom(ctx, room); err != nil {
		return err
	}
	return rs.cache.SetRoom(ctx, room.ID, room)
}

func (rs *RoomService) DeleteRoom(ctx context.Context, id string) error {
	if err := rs.storage.DeleteRoom(ctx, id); err != nil {
		return err
	}
	return rs.cache.DeleteRoom(ctx, id)
}

func (rs *RoomService) ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	return rs.storage.ListUserRooms(ctx, userID)
}
```

internal/chat/service/userClientService.go
```
package service

import (
	"context"
	"fmt"
	"krampus/internal/chat/storage"
	userDomain "krampus/internal/user/domain"
	"krampus/pkg/apperror"
	"log"
	"os/user"
	"time"
)

type UserClientStorage interface {
	SaveUserClient(ctx context.Context, user *user.User) error
	GetUserClient(ctx context.Context, id string) (*user.User, error)
	UpdateUserClient(ctx context.Context, user *user.User) error
	UpdateLastActivity(ctx context.Context, userID string, ts int64) error
}

type UserClientCache interface {
	GetUserClient(ctx context.Context, id string) (*user.User, error)
	SetUserClient(ctx context.Context, id string, user *user.User) error
	DeleteUserClient(ctx context.Context, id string) error
}

type UserClientService struct {
	storage *storage.UserClientPGStorage
	cache   *storage.UserClientCache
}

func NewUserClientService(s *storage.UserClientPGStorage, c *storage.UserClientCache) *UserClientService {
	return &UserClientService{
		storage: s,
		cache:   c,
	}
}

func (ucs *UserClientService) GetUser(ctx context.Context, id string) (*userDomain.User, error) {
	if user, err := ucs.cache.GetUserClient(ctx, id); err == nil && user != nil {
		return user, nil
	}

	user, err := ucs.storage.GetUserClient(ctx, id)
	if err != nil {
		return nil, apperror.New(apperror.ErrUserNotFound, "user not found")
	}

	go ucs.cache.SetUserClient(ctx, id, user)
	return user, nil
}

func (ucs *UserClientService) UpdateLastActivity(ctx context.Context, userID string) {
	now := time.Now().UnixNano()

	if err := ucs.storage.UpdateLastActivity(ctx, userID, now); err != nil {
		log.Printf("Failed to update activity for %s: %v", userID, err)
		return
	}

	if user, err := ucs.cache.GetUserClient(ctx, userID); err == nil {
		user.LastActive = now
		go ucs.cache.SetUserClient(ctx, userID, user)
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

func (ucs *UserClientService) GetUserStatus(userID string) userDomain.ChatUserStatus {
	user, err := ucs.GetUser(context.Background(), userID)
	if err != nil {
		return userDomain.StatusOffline
	}

	inactiveDuration := time.Since(time.Unix(0, user.LastActive))

	switch {
	case inactiveDuration < 5*time.Minute:
		return userDomain.StatusOnline
	case inactiveDuration < 30*time.Minute:
		return userDomain.StatusAway
	default:
		return userDomain.StatusOffline
	}
}

func (ucs *UserClientService) SaveUser(ctx context.Context, user *userDomain.User) error {
	if err := ucs.storage.SaveUserClient(ctx, user); err != nil {
		return err
	}
	idStr := string(user.ID)
	return ucs.cache.SetUserClient(ctx, idStr, user)
}

func (ucs *UserClientService) UpdateUser(ctx context.Context, user *userDomain.User) error {
	if err := ucs.storage.UpdateUserClient(ctx, user); err != nil {
		return err
	}
	idStr := string(user.ID)
	return ucs.cache.DeleteUserClient(ctx, idStr)
}
```

internal/chat/storage/roomStorage.go
```
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"krampus/internal/chat/domain"
	database "krampus/internal/sqlc"
)

type RoomPGStorage struct {
	queries *database.Queries
}

func NewRoomPGStorage(queries *database.Queries) *RoomPGStorage {
	return &RoomPGStorage{queries: queries}
}

func (s *RoomPGStorage) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	row, err := s.queries.GetRoomByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	var members []string
	if err := json.Unmarshal(row.Members, &members); err != nil {
		return nil, fmt.Errorf("failed to unmarshal members: %w", err)
	}

	var settings domain.RoomSettings
	if err := json.Unmarshal(row.Settings, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &domain.Room{
		ID:        row.ID,
		Type:      domain.RoomType(row.Type),
		OwnerID:   row.OwnerID,
		Name:      row.Name,
		Members:   members,
		Settings:  settings,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *RoomPGStorage) SaveRoom(ctx context.Context, room *domain.Room) error {
	membersJSON, err := json.Marshal(room.Members)
	if err != nil {
		return err
	}
	settingsJSON, err := json.Marshal(room.Settings)
	if err != nil {
		return err
	}

	return s.queries.UpsertRoom(ctx, database.UpsertRoomParams{
		ID:        room.ID,
		Type:      string(room.Type),
		OwnerID:   room.OwnerID,
		Name:      room.Name,
		Members:   membersJSON,
		Settings:  settingsJSON,
		CreatedAt: room.CreatedAt,
		UpdatedAt: room.UpdatedAt,
	})
}

func (s *RoomPGStorage) UpdateRoom(ctx context.Context, room *domain.Room) error {
	membersJSON, err := json.Marshal(room.Members)
	if err != nil {
		return err
	}
	settingsJSON, err := json.Marshal(room.Settings)
	if err != nil {
		return err
	}

	return s.queries.UpdateRoom(ctx, database.UpdateRoomParams{
		ID:        room.ID,
		Name:      room.Name,
		Members:   membersJSON,
		Settings:  settingsJSON,
		UpdatedAt: time.Now().UnixNano(),
	})
}

func (s *RoomPGStorage) DeleteRoom(ctx context.Context, id string) error {
	return s.queries.DeleteRoom(ctx, id)
}

func (s *RoomPGStorage) ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	arg := fmt.Sprintf("[\"%s\"]", userID)
	rows, err := s.queries.ListUserRooms(ctx, []byte(arg))
	if err != nil {
		return nil, err
	}

	var rooms []*domain.Room
	for _, row := range rows {
		var members []string
		_ = json.Unmarshal(row.Members, &members)
		var settings domain.RoomSettings
		_ = json.Unmarshal(row.Settings, &settings)

		rooms = append(rooms, &domain.Room{
			ID:        row.ID,
			Type:      domain.RoomType(row.Type),
			OwnerID:   row.OwnerID,
			Name:      row.Name,
			Members:   members,
			Settings:  settings,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return rooms, nil
}
```

internal/chat/storage/roomCache.go
```
// internal/chat/storage/roomCache.go - Заполнен пустой
package storage

import (
	"context"
	"encoding/json"
	"time"

	"krampus/internal/chat/domain"
	myredis "krampus/pkg/client-database/redis"

	"github.com/redis/go-redis/v9"
)

type RoomCache struct {
	client *myredis.Client
}

func NewRoomCache(client *myredis.Client) *RoomCache {
	return &RoomCache{client: client}
}

func (c *RoomCache) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	data, err := c.client.RDB().Get(ctx, "room:"+id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var room domain.Room
	if err := json.Unmarshal([]byte(data), &room); err != nil {
		return nil, err
	}
	return &room, nil
}

func (c *RoomCache) SetRoom(ctx context.Context, id string, room *domain.Room) error {
	data, err := json.Marshal(room)
	if err != nil {
		return err
	}
	return c.client.RDB().Set(ctx, "room:"+id, data, 10*time.Minute).Err()
}

func (c *RoomCache) DeleteRoom(ctx context.Context, id string) error {
	return c.client.RDB().Del(ctx, "room:"+id).Err()
}
```

internal/chat/storage/userClientStorage.go
```
package storage

import (
	"context"
	"fmt"
	"strconv"
	"time"

	database "krampus/internal/sqlc"
	"krampus/internal/user/domain"

	"github.com/jackc/pgx/v5/pgtype"
)

type UserClientPGStorage struct {
	queries *database.Queries
}

func NewUserClientPGStorage(queries *database.Queries) *UserClientPGStorage {
	return &UserClientPGStorage{queries: queries}
}

func (s *UserClientPGStorage) GetUserClient(ctx context.Context, id string) (*domain.User, error) {
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user id format: %w", err)
	}

	user, err := s.queries.GetUserByID(ctx, idInt)
	if err != nil {
		return nil, err
	}
	return &domain.User{
		ID:         user.ID,
		Username:   user.Username,
		Email:      user.Email,
		LastActive: user.CreatedAt.Time.UnixNano(),
	}, nil
}

func (s *UserClientPGStorage) UpdateLastActivity(ctx context.Context, userID string, timestamp int64) error {
	idInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return err
	}

	updateTime := time.Unix(0, timestamp)

	return s.queries.UpdateUserLastActive(ctx, database.UpdateUserLastActiveParams{
		ID:        idInt,
		UpdatedAt: pgtype.Timestamptz{Time: updateTime, Valid: true},
	})
}

func (s *UserClientPGStorage) SaveUserClient(ctx context.Context, user *domain.User) error {
	return s.queries.CreateUserClient(ctx, database.CreateUserClientParams{
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: "system_generated",
	})
}

func (s *UserClientPGStorage) UpdateUserClient(ctx context.Context, user *domain.User) error {
	return s.queries.UpdateUserClient(ctx, database.UpdateUserClientParams{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
}
```

internal/chat/storage/userClientCache.go
```
package storage

import (
	"context"
	"encoding/json"
	"time"

	"krampus/internal/user/domain"
	myredis "krampus/pkg/client-database/redis"

	"github.com/redis/go-redis/v9"
)

type UserClientCache struct {
	client *myredis.Client
}

func NewUserClientCache(client *myredis.Client) *UserClientCache {
	return &UserClientCache{client: client}
}

func (c *UserClientCache) GetUserClient(ctx context.Context, id string) (*domain.User, error) {
	data, err := c.client.RDB().Get(ctx, "user_client:"+id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var user domain.User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *UserClientCache) SetUserClient(ctx context.Context, id string, user *domain.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return nil
	}
	return c.client.RDB().Set(ctx, "user_client:"+id, data, 5*time.Minute).Err()
}

func (c *UserClientCache) DeleteUserClient(ctx context.Context, id string) error {
	return c.client.RDB().Del(ctx, "user_client:"+id).Err()
}
```

internal/message/adapters/websocketClient.go
```
package adapters

import (
	"encoding/json"
	"fmt"
	message "krampus/internal/message/domain"
	"krampus/internal/message/service"
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
	Send   chan *message.BaseMessage
	mu     sync.Mutex
	msgSvc *service.MessageService
}

func NewClient(conn *websocket.Conn, userID, roomID string, svc *service.MessageService) *Client {
	return &Client{
		conn:   conn,
		UserID: userID,
		RoomID: roomID,
		Send:   make(chan *message.BaseMessage, 256),
		msgSvc: svc,
	}
}

func (c *Client) ConnID() string {
	return c.conn.RemoteAddr().String() + "-" + c.UserID
}

func (c *Client) Start() {
	go c.writePump()
	go c.readPump()
}

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
		var msg message.BaseMessage
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

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for message := range c.Send {
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

func (c *Client) SendError(appErr *apperror.AppError) {
	errorMsg := &message.BaseMessage{
		Type:    "error",
		Payload: json.RawMessage(fmt.Sprintf(`{"code": "%s", "message": "%s}`, appErr.Code, appErr.Message)),
	}
	select {
	case c.Send <- errorMsg:
	default:
	}
}
```

internal/message/adapters/websocketManager.go
```
package adapters

import (
	"context"
	"log"
	"sync"

	message "krampus/internal/message/domain"
	"krampus/pkg/messaging/kafka"
)

type ConnectionManager struct {
	users         sync.Map
	rooms         sync.Map
	userLocks     sync.Map
	kafkaConsumer *kafka.Consumer
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

func NewConnectionManager(kafkaConsumer *kafka.Consumer) *ConnectionManager {
	m := &ConnectionManager{
		kafkaConsumer: kafkaConsumer,
	}
	m.kafkaConsumer.AddHandler(func(msg *message.BaseMessage) {
		m.BroadcastToRoom(msg.RoomID, msg)
	})

	go m.kafkaConsumer.Consume(context.Background())

	return m
}

func (m *ConnectionManager) Register(client *Client) error {
	userID, roomID := client.UserID, client.RoomID

	ucI, _ := m.users.LoadOrStore(userID, &UserConnections{
		conns:  make(map[string]*Client),
		userID: userID,
	})
	uc := ucI.(*UserConnections)

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
		uc := ucI.(*UserConnections)
		LockI, ok := m.userLocks.Load(userID)
		if !ok {
			return
		}

		userLock := LockI.(*sync.Mutex)

		userLock.Lock()
		defer userLock.Unlock()

		uc.mu.Lock()
		delete(uc.conns, client.ConnID())
		uc.mu.Unlock()

		if len(uc.conns) == 0 {
			m.unsubscribeFromRoom(roomID, userID)
			m.users.Delete(userID)
			m.userLocks.Delete(userID)
		}
	}
}

func (m *ConnectionManager) BroadcastToRoom(roomID string, msg *message.BaseMessage) error {
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

func (m *ConnectionManager) SendToUser(userID string, msg *message.BaseMessage) error {
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

internal/message/adapters/websocketServer.go
```
package adapters

import (
	"log"
	"net/http"

	"krampus/internal/message/domain"
	"krampus/internal/message/service"
	"krampus/pkg/config"
	"krampus/pkg/messaging/kafka"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WebSocketServer struct {
	service       *service.MessageService
	config        *config.Config
	connectionMgr *ConnectionManager
}

func NewWebSocketServer(s *service.MessageService, cfg *config.Config, kafkaConsumer *kafka.Consumer) *WebSocketServer {
	return &WebSocketServer{
		service:       s,
		config:        cfg,
		connectionMgr: NewConnectionManager(kafkaConsumer),
	}
}

// func (w *WebSocketServer) Start() {
// 	log.Printf("WS server starting on port %s", w.config.HTTPPort)
// 	mux := http.NewServeMux()
// 	mux.HandleFunc("/ws", w.HandleWebSocket)

// 	if err := http.ListenAndServe(w.config.HTTPPort, mux); err != nil {
// 		log.Fatalf("WS server failed: %v", err)
// 	}
// }

func (w *WebSocketServer) HandleWebSocket(wr http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(wr, r, nil)
	if err != nil {
		println("Upgrade err:", err)
		return
	}

	userID := r.URL.Query().Get("user_id")
	roomID := r.URL.Query().Get("room_id")
	token := r.URL.Query().Get("token")

	if userID == "" || roomID == "" || token == "" {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "missing params"))
		conn.Close()
		return
	}

	if token != "valid" {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid token"))
		conn.Close()
		return
	}

	client := NewClient(conn, userID, roomID, w.service)

	// Регистрируем клиента в менеджере
	if err := w.connectionMgr.Register(client); err != nil {
		log.Printf("Failed to register client: %v", err)
		conn.Close()
		return
	}

	defer w.connectionMgr.Unregister(client)

	log.Printf("WS connected: %s @ %s", userID, roomID)

	for {
		var msg domain.BaseMessage
		err := conn.ReadJSON(&msg)

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS read error: %v", err)
			}
			break
		}

		msg.UserID = userID
		msg.RoomID = roomID

		if err := w.service.Process(r.Context(), &msg); err != nil {
			log.Printf("Message process error: %v", err)
			continue
		}

		w.connectionMgr.BroadcastToRoom(roomID, &msg)
	}
}
```

internal/message/domain/message.go
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

internal/message/service/messageService.go
```
package service

import (
	"context"
	chat "krampus/internal/chat/service"
	message "krampus/internal/message/domain"
	messageStorage "krampus/internal/message/storage"
	"krampus/pkg/apperror"
	"log"
	"time"
)

type MessageStorage interface {
	SaveMessage(ctx context.Context, msg *message.BaseMessage) error
	SaveMessageBatch(ctx context.Context, msgs []*message.BaseMessage) error
	GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*message.BaseMessage, error)
	GetMessage(ctx context.Context, id string) (*message.BaseMessage, error)
}

type MessageDistributor interface {
	Broadcast(ctx context.Context, msg *message.BaseMessage) error
	BroadcastToRoom(ctx context.Context, msg *message.BaseMessage, roomID string) error
	SendToUserClient(ctx context.Context, userID string, msg *message.BaseMessage) error
}

type MessageService struct {
	storage       *messageStorage.MessagePGStorage
	distributor   *messageStorage.MessageDistributor
	roomSvc       *chat.RoomService
	userClientSvc *chat.UserClientService
	rateLimiter   *RateLimiter
}

func NewMessageService(storage *messageStorage.MessagePGStorage, dist *messageStorage.MessageDistributor, roomSvc *chat.RoomService, userClientSvc *chat.UserClientService) *MessageService {
	return &MessageService{
		storage:       storage,
		distributor:   dist,
		roomSvc:       roomSvc,
		userClientSvc: userClientSvc,
		rateLimiter:   NewRateLimiter(),
	}
}

func (ms *MessageService) Process(ctx context.Context, msg *message.BaseMessage) error {
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
	if !ms.roomSvc.CanSendMessage(ctx, room, user, msg.Type) {
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

func (ms *MessageService) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*message.BaseMessage, error) {
	if _, err := ms.roomSvc.GetRoom(ctx, roomID); err != nil {
		return nil, apperror.New(apperror.ErrRoomNotFound, "room not found")
	}

	return ms.storage.GetRoomMessages(ctx, roomID, limit)
}

func (ms *MessageService) GetMessage(ctx context.Context, id string) (*message.BaseMessage, error) {
	msg, err := ms.storage.GetMessage(ctx, id)
	if err != nil {
		return nil, apperror.New(apperror.ErrInternal, "failed to get message")
	}
	return msg, nil
}

func (ms *MessageService) SaveMessageBatch(ctx context.Context, msgs []*message.BaseMessage) error {
	return ms.storage.SaveMessageBatch(ctx, msgs)
}

func (ms *MessageService) BroadcastToRoom(ctx context.Context, msg *message.BaseMessage, roomID string) error {
	msg.RoomID = roomID
	return ms.distributor.Broadcast(ctx, msg)
}

func (ms *MessageService) SendToUserClient(ctx context.Context, userID string, msg *message.BaseMessage) error {
	msg.UserID = userID
	return ms.distributor.Broadcast(ctx, msg)
}
```

internal/message/service/rateLimiterService.go
```
package service

import (
	"context"
	"krampus/internal/message/domain"
	"krampus/pkg/apperror"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	lastUsed map[string]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		lastUsed: make(map[string]time.Time),
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
	rl.lastUsed[key] = time.Now()
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
			delete(rl.lastUsed, key)
		}
	}
}
```

internal/message/storage/fileStorage.go
```
package storage

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	chatDomain "krampus/internal/chat/domain"
	chat "krampus/internal/chat/service"
	"krampus/internal/message/domain"
)

type FileStorage struct {
	basePath    string
	segmentSize time.Duration
	buffers     sync.Map // roomID → *RoomBuffer
	roomSvc     *chat.RoomService
}

type RoomBuffer struct {
	mu         sync.Mutex
	messages   []*domain.BaseMessage
	size       int64
	lastFlush  time.Time
	activeFile *os.File
	writer     *bufio.Writer
}

func NewFileStorage(basePath string, segmentSize time.Duration, roomSvc *chat.RoomService) *FileStorage {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		log.Printf("Failed to create base directory: %v", err)
	}
	return &FileStorage{
		basePath:    basePath,
		segmentSize: segmentSize,
		roomSvc:     roomSvc,
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
	case chatDomain.RoomVideoCall:
		// Видеозвонки: 1ч сегменты
		return filepath.Join(f.basePath, "video_calls", roomID,
			t.Format("2006-01-02"), fmt.Sprintf("%d.log", t.Hour()))

	case chatDomain.RoomGroup:
		// Групповые: 4ч сегменты
		hour := (t.Hour() / 4) * 4
		return filepath.Join(f.basePath, "groups", roomID,
			t.Format("2006-01"), fmt.Sprintf("%s_%02d.log", t.Format("2006-01-02"), hour))

	case chatDomain.RoomPrivate:
		// Личные: 1 день + шардинг
		shard := roomID[:2]
		return filepath.Join(f.basePath, "private", shard, roomID,
			t.Format("2006-01-02")+".log")

	case chatDomain.RoomPersonal:
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

func (f *FileStorage) getRoomType(roomID string) chatDomain.RoomType {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	room, err := f.roomSvc.GetRoom(ctx, roomID)
	if err != nil || room == nil {
		if len(roomID) > 0 {
			switch roomID[0] {
			case 'u':
				return chatDomain.RoomPrivate
			case 'v':
				return chatDomain.RoomVideoCall
			case 'p':
				return chatDomain.RoomPersonal
			}
		}
		return chatDomain.RoomGroup
	}

	return room.Type
}
```

internal/message/storage/MessageDistributor.go
```
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"krampus/internal/message/domain"
	"krampus/pkg/config"
	"krampus/pkg/logging"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type MessageDistributor struct {
	producer *kafka.Producer
	logger   logging.Logger
	topic    string
}

type KafkaMessage struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	UserID    string          `json:"user_id"`
	RoomID    string          `json:"room_id"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

func NewMessageDistributor(cfg config.KafkaConfig, logger logging.Logger) *MessageDistributor {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(cfg.Brokers, ","),
		"client.id":          "krampus-msg-distributor",
		"acks":               "all",
		"enable.idempotence": true,
	})
	if err != nil {
		logger.Fatalf("Kafka producer failed: %v", err)
	}

	return &MessageDistributor{
		producer: p,
		logger:   logger,
		topic:    cfg.Topics.Incoming,
	}
}

// BroadcastToRoom — отправка в конкретную комнату (реализация идентична Broadcast, но с подменой темы если нужно)
func (d *MessageDistributor) BroadcastToRoom(ctx context.Context, msg *domain.BaseMessage, roomID string) error {
	msg.RoomID = roomID
	return d.Broadcast(ctx, msg)
}

// SendToUserClient — отправка персонального сообщения (например, в топик уведомлений)
func (d *MessageDistributor) SendToUserClient(ctx context.Context, userID string, msg *domain.BaseMessage) error {
	msg.UserID = userID
	return d.Broadcast(ctx, msg)
}

func (d *MessageDistributor) Broadcast(ctx context.Context, msg *domain.BaseMessage) error {
	event := KafkaMessage{
		ID:        msg.ID,
		Type:      string(msg.Type),
		UserID:    msg.UserID,
		RoomID:    msg.RoomID,
		Timestamp: msg.Timestamp,
		Payload:   msg.Payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	deliveryChan := make(chan kafka.Event, 1)

	err = d.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &d.topic, Partition: kafka.PartitionAny},
		Value:          data,
		Key:            []byte(msg.RoomID),
	}, deliveryChan)

	if err != nil {
		return fmt.Errorf("kafka produce error: %w", err)
	}

	go func() {
		e := <-deliveryChan
		m := e.(*kafka.Message)

		if m.TopicPartition.Error != nil {
			d.logger.Errorf("Failed to deliver message %s: %v", msg.ID, m.TopicPartition.Error)
		} else {
			d.logger.Infof("Message %s delivered [partition %d, offset %v]", msg.ID, m.TopicPartition.Partition, m.TopicPartition.Offset)
		}
		close(deliveryChan)
	}()

	return nil
}

func (d *MessageDistributor) Close() {
	d.producer.Flush(15 * 1000)
	d.producer.Close()
}
```

internal/message/storage/psqlStorage.go
```
package storage

import (
	"context"
	"fmt"

	"krampus/internal/message/domain"
	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessagePGStorage struct {
	pool    *pgxpool.Pool
	queries *database.Queries
}

func NewMessagePGStorage(pool *pgxpool.Pool, queries *database.Queries) *MessagePGStorage {
	return &MessagePGStorage{
		pool:    pool,
		queries: queries,
	}
}

// SaveMessage — сохранение одного сообщения через sqlc
func (p *MessagePGStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
	return p.queries.SaveMessage(ctx, database.SaveMessageParams{
		ID:        msg.ID,
		Type:      string(msg.Type),
		UserID:    msg.UserID,
		RoomID:    msg.RoomID,
		Timestamp: msg.Timestamp,
		Payload:   msg.Payload,
		Signature: pgtype.Text{String: msg.Signature, Valid: msg.Signature != ""},
	})
}

// SaveMessageBatch — пакетное сохранение через транзакцию sqlc
func (p *MessagePGStorage) SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error {
	// Создаем транзакцию, используя переданный пул
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// "Привязываем" запросы к этой транзакции
	qtx := p.queries.WithTx(tx)

	for _, msg := range msgs {
		err := qtx.SaveMessage(ctx, database.SaveMessageParams{
			ID:        msg.ID,
			Type:      string(msg.Type),
			UserID:    msg.UserID,
			RoomID:    msg.RoomID,
			Timestamp: msg.Timestamp,
			Payload:   msg.Payload,
			Signature: pgtype.Text{String: msg.Signature, Valid: msg.Signature != ""},
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetRoomMessages — история чата через sqlc
func (p *MessagePGStorage) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error) {
	rows, err := p.queries.GetRoomMessages(ctx, database.GetRoomMessagesParams{
		RoomID: roomID,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get room messages: %w", err)
	}

	messages := make([]*domain.BaseMessage, 0, len(rows))
	for _, row := range rows {
		var ts int64
		if row.CreatedAt.Valid {
			ts = row.CreatedAt.Time.UnixNano()
		}

		messages = append(messages, &domain.BaseMessage{
			ID:        row.ID,
			Type:      domain.MessageType(row.Type),
			UserID:    row.UserID,
			RoomID:    row.RoomID,
			Timestamp: ts,
			Payload:   row.Payload,
			Signature: row.Signature.String,
		})
	}

	// Реверс для хронологического порядка
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetMessage — получение одного сообщения по ID
func (p *MessagePGStorage) GetMessage(ctx context.Context, id string) (*domain.BaseMessage, error) {
	row, err := p.queries.GetMessage(ctx, id)
	if err != nil {
		return nil, err
	}

	var ts int64
	if row.CreatedAt.Valid {
		ts = row.CreatedAt.Time.UnixNano()
	}

	return &domain.BaseMessage{
		ID:        row.ID,
		Type:      domain.MessageType(row.Type),
		UserID:    row.UserID,
		RoomID:    row.RoomID,
		Timestamp: ts,
		Payload:   row.Payload,
		Signature: row.Signature.String,
	}, nil
}

// CleanupOldMessages — удаление старых сообщений каждые 24ч
func (p *MessagePGStorage) CleanupOldMessages(ctx context.Context) error {
	return p.queries.CleanupOldMessages(ctx)
}

func (p *MessagePGStorage) Close() {
	p.pool.Close()
}
```

internal/message/storage/redisStorage.go
```
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	chatDomain "krampus/internal/chat/domain"
	"krampus/internal/message/domain"
	userDomain "krampus/internal/user/domain"

	"github.com/redis/go-redis/v9"
)

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
func (r *RedisStorage) SetUserConnection(ctx context.Context, userID string, conn *chatDomain.UserConnection) error {
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
func (r *RedisStorage) GetRoom(ctx context.Context, id string) (*chatDomain.Room, error) {
	key := "room:" + id
	roomJSON, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room from cache: %w", err)
	}

	var room chatDomain.Room
	if err := json.Unmarshal([]byte(roomJSON), &room); err != nil {
		return nil, fmt.Errorf("failed to unmarshal room: %w", err)
	}
	return &room, nil
}

func (r *RedisStorage) SetRoom(ctx context.Context, id string, room *chatDomain.Room) error {
	key := "room:" + id
	roomJSON, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("failed to marshal room: %w", err)
	}
	return r.client.Set(ctx, key, roomJSON, 10*time.Minute).Err()
}

func (r *RedisStorage) GetAccessToken(token string) (*userDomain.CachedSession, error) {
	key := "access_token:" + token
	data, err := r.client.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}

	var session userDomain.CachedSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *RedisStorage) SetAccessToken(token string, session userDomain.CachedSession) error {
	key := "access_token:" + token
	data, _ := json.Marshal(session)
	expiration := time.Until(session.ExpiresAt)
	return r.client.Set(context.Background(), key, data, expiration).Err()
}
```

internal/user/adapters/refreshTokenHandler.go
```
package adapters

import (
	"krampus/internal/user/domain"
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

func (h *refreshTokenHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/refresh", h.refresh)
	// rg.POST("/send-code", h.sendEmailToken) // раскомментируй, когда сервис будет готов
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

internal/user/adapters/userHandler.go
```
package adapters

import (
	"krampus/internal/user/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/logging"
	"net/http"

	twofa "krampus/internal/auth/domain"

	"github.com/gin-gonic/gin"
)

type UserService interface {
	UserRegister(users domain.User) (domain.User, error)
	UserLogin(users domain.User) (domain.TokenResponse, twofa.TwoFaCodes, error)
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

func (h *userHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/signup", h.signUp)
	rg.POST("/signin", h.signIn)
	rg.POST("/logout", h.logout)
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

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
```

internal/user/domain/session.go
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

internal/user/domain/user.go
```
package domain

import "time"

type ChatUserStatus string

const (
	StatusOnline  ChatUserStatus = "online"
	StatusAway    ChatUserStatus = "away"
	StatusOffline ChatUserStatus = "offline"
	StatusDND     ChatUserStatus = "dnd"
)

type User struct {
	ID           int64                  `json:"id"`
	Username     string                 `json:"username"`
	Firstname    string                 `json:"firstname"`
	Lastname     string                 `json:"lastname"`
	Email        string                 `json:"email"`
	Password     string                 `json:"password"`
	PasswordHash string                 `json:"-"`
	TwoFAEnabled bool                   `json:"two_fa_enabled"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActive   int64                  `json:"last_active"`
	Status       ChatUserStatus         `json:"status"`
	Permissions  []string               `json:"permissions"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	// BlockedUntil
	// FailedLogins
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

internal/user/service/refreshTokenService.go
```
package service

import (
	"errors"
	"time"

	"krampus/internal/user/domain"

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

func NewRefreshToken(refreshToken RefreshTokenStorage, jwt string) *RefreshToken {
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

internal/user/service/userService.go
```
package service

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"

	twofa "krampus/internal/auth/domain"
	"krampus/internal/user/domain"
	redis "krampus/internal/user/storage"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserStorage interface {
	InsertUser(user domain.User) (int64, error)
	SelectUserByEmail(email string) (domain.User, error)
	SelectUserByID(userID int64) (domain.User, error)
	BlockUser(email, blockedUntil string) error
	// RedisSessionStorage() redis.SessionStorage
}

type LoginAttemptStorage interface {
	LogAttempt(email string, result bool, attemptTime time.Time) error
	GetFailedLogAttempts(email string, windowStart time.Time) (int64, error)
	UserBlocked(email string, windowStart time.Time) ([]map[string]interface{}, error)
}

type User struct {
	userStorage         UserStorage
	loginAttemptStorage LoginAttemptStorage
	refreshTokenService *RefreshToken
	redisStorage        *redis.RedisSessionStorage
	jwtSecret           string
}

func NewUser(
	user UserStorage,
	loginAttempt LoginAttemptStorage,
	refreshToken *RefreshToken,
	redisStorage *redis.RedisSessionStorage,
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

func (s *User) UserLogin(user domain.User) (domain.TokenResponse, twofa.TwoFaCodes, error) {
	fmt.Printf("DEBUG LOGIN: Attempting login for email: '%s'\n", user.Email)
	fmt.Printf("DEBUG LOGIN: Password provided: '%s'\n", user.Password)
	fmt.Printf("DEBUG LOGIN: TwoFA enabled: '%v'\n", user.TwoFAEnabled)

	if user.Email == "" || user.Password == "" {
		fmt.Printf("DEBUG LOGIN: Email or password empty\n")
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("email and password are required")
	}

	blocked, minutesLeft, err := s.IsUserBlocked(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error checking block status: %v\n", err)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	if blocked {
		fmt.Printf("DEBUG LOGIN: User is blocked for %d minutes\n", minutesLeft)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, fmt.Errorf("your account is blocked for %d minutes", minutesLeft)
	}

	fmt.Printf("DEBUG LOGIN: Searching user in database...\n")
	dbUser, err := s.userStorage.SelectUserByEmail(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Database error or user not found: %v\n", err)
		s.LogLoginAttempt(user.Email, false)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGIN: User found - ID: %d, Email: %s\n", dbUser.ID, dbUser.Email)
	fmt.Printf("DEBUG LOGIN: Stored password hash: %s\n", dbUser.PasswordHash)
	fmt.Printf("DEBUG LOGIN: Provided password: %s\n", user.Password)

	fmt.Printf("DEBUG LOGIN: Comparing passwords...\n")
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(user.Password))
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Password comparison failed: %v\n", err)
		s.LogLoginAttempt(user.Email, false)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGIN: Password correct!\n")

	attempts, err := s.GetFailedAttempts(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error getting failed attempts: %v\n", err)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	maxAttempts := int64(5)
	if attempts >= maxAttempts {
		fmt.Printf("DEBUG LOGIN: Too many failed attempts: %d\n", attempts)
		s.BlockUser(user.Email)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("too many failed attempts, account blocked")
	}

	// if dbUser.TwoFAEnabled {
	if dbUser.TwoFAEnabled != false {
		tempToken, err := s.GenerateTempToken(dbUser.ID)
		if err != nil {
			fmt.Printf("DEBUG LOGIN: Error generating temp token: %v\n", err)
			return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
		}
		return domain.TokenResponse{}, twofa.TwoFaCodes{RequiresTwoFa: true, TempToken: tempToken}, nil
	}

	accessToken, err := s.GenerateAccessToken(dbUser.ID)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error generating access token: %v\n", err)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	refreshToken, err := s.refreshTokenService.GenerateRefreshToken(dbUser.ID)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error generating refresh token: %v\n", err)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
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
	return domain.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}, twofa.TwoFaCodes{}, nil
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

internal/user/storage/loginAttempt_storage.go
```
package storage

import (
	"context"
	"time"

	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type LoginAttemptStorageError struct {
	Message string
}

type LoginAttemptStorage struct {
	queries *database.Queries
}

var (
	ErrLoginAttemptExpired = &LoginAttemptStorageError{"token expired"}
)

func NewLoginAttemptStorage(queries *database.Queries) *LoginAttemptStorage {
	return &LoginAttemptStorage{queries: queries}
}

func (e *LoginAttemptStorageError) LoginAttemptError() string {
	return e.Message
}

func (s *LoginAttemptStorage) LogAttempt(email string, result bool, attemptTime time.Time) error {
	params := database.CreateLoginAttemptParams{
		Email:       email,
		Success:     result,
		AttemptedAt: pgtype.Timestamptz{Time: attemptTime, Valid: true},
	}

	return s.queries.CreateLoginAttempt(context.Background(), params)
}

func (s *LoginAttemptStorage) GetFailedLogAttempts(email string, windowStart time.Time) (int64, error) {
	count, err := s.queries.GetRecentFailedAttempts(context.Background(), database.GetRecentFailedAttemptsParams{
		Email:       email,
		AttemptedAt: pgtype.Timestamptz{Time: windowStart, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *LoginAttemptStorage) UserBlocked(email string, windowStart time.Time) ([]map[string]interface{}, error) {
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

internal/user/storage/psql_user_storage.go
```
package storage

import (
	"context"
	"time"

	database "krampus/internal/sqlc"
	"krampus/internal/user/domain"

	"github.com/jackc/pgx/v5/pgtype"
)

type UserStorageError struct {
	Message string
}

type UserStorage struct {
	queries *database.Queries
}

var (
	ErrTokenExpired = &UserStorageError{"token expired"}
)

func NewUserStorage(queries *database.Queries) *UserStorage {
	return &UserStorage{queries: queries}
}

func (e *UserStorageError) UserError() string {
	return e.Message
}

func (s *UserStorage) InsertUser(user domain.User) (int64, error) {
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

func (s *UserStorage) SelectUserByEmail(email string) (domain.User, error) {
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

func (s *UserStorage) SelectUserByID(id int64) (domain.User, error) {
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

func (s *UserStorage) BlockUser(email, blockedUntil string) error {
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

internal/user/storage/redis_session_storage.go
```
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"krampus/internal/user/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisSessionStorage struct {
	client *redis.Client
}

func NewRedisSessionStorage(client *redis.Client) *RedisSessionStorage {
	return &RedisSessionStorage{
		client: client,
	}
}

func (s *RedisSessionStorage) SetAccessToken(token string, session domain.CachedSession) error {
	key := fmt.Sprintf("access: %s", token)

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	return s.client.Set(context.Background(), key, data, ttl).Err()
}

func (s *RedisSessionStorage) GetAccessToken(token string) (domain.CachedSession, error) {
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

func (s *RedisSessionStorage) DeleteAccessToken(token string) error {
	key := fmt.Sprintf("access: %s", token)
	return s.client.Del(context.Background(), key).Err()
}

func (s *RedisSessionStorage) GetSessionByUserID(userID int64) (domain.CachedTempToken, error) {
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

func (s *RedisSessionStorage) DeleteSessionByUserID(userID int64) error {
	key := fmt.Sprintf("session:%d", userID)
	return s.client.Del(context.Background(), key).Err()
}

func (s *RedisSessionStorage) SetSessionByUserID(userID int64, session domain.CachedSession) error {
	key := fmt.Sprintf("session:%d", userID)
	data, _ := json.Marshal(session)
	return s.client.Set(context.Background(), key, data, 7*24*time.Hour).Err()
}

func (s *RedisSessionStorage) SetTempToken(token string, temp domain.CachedTempToken) error {
	key := fmt.Sprintf("temp:%s", token)
	data, _ := json.Marshal(temp)
	ttl := time.Until(temp.ExpiresAt)
	return s.client.Set(context.Background(), key, data, ttl).Err()
}

func (s *RedisSessionStorage) GetTempToken(token string) (domain.CachedTempToken, error) {
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

func (s *RedisSessionStorage) RedisSessionStorage() {
	// Оставляем пустым, если реальная логика не требуется,
	// но метод нужен для реализации интерфейса
}
```

internal/user/storage/refreshToken_storage.go
```
package storage

import (
	"context"
	"errors"
	"time"

	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type RefreshTokenStorageError struct {
	Message string
}

type RefreshTokenStorage struct {
	queries *database.Queries
}

var (
	ErrRefreshTokenExpired = errors.New("token expired")
)

func NewRefreshTokenStorage(queries *database.Queries) *RefreshTokenStorage {
	return &RefreshTokenStorage{queries: queries}
}

func (e *RefreshTokenStorageError) RefreshTokenError() string {
	return e.Message
}

func (s *RefreshTokenStorage) RefreshStore(userID int64, token string, expiresAt time.Time) error {
	params := database.CreateRefreshTokenParams{
		UserID:    userID,
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}

	return s.queries.CreateRefreshToken(context.Background(), params)
}

func (s *RefreshTokenStorage) RefreshGet(token string) (int64, error) {
	refreshToken, err := s.queries.GetRefreshToken(context.Background(), token)
	if err != nil {
		return 0, err
	}

	if time.Now().After(refreshToken.ExpiresAt.Time) {
		s.RefreshDelete(token)
		return 0, ErrRefreshTokenExpired
	}

	return refreshToken.UserID, nil
}

func (s *RefreshTokenStorage) RefreshDelete(token string) error {
	return s.queries.DeleteRefreshToken(context.Background(), token)
}

func (s *RefreshTokenStorage) RefreshDeleteByUserID(userID int64) error {
	return s.queries.RefreshDeleteByUserID(context.Background(), userID)
}
```

internal/sqlc/db.go
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1

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

internal/sqlc/models.go
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1

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

type Message struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	UserID    string             `json:"user_id"`
	RoomID    string             `json:"room_id"`
	Timestamp int64              `json:"timestamp"`
	Payload   []byte             `json:"payload"`
	Signature pgtype.Text        `json:"signature"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
}

type RefreshToken struct {
	ID        int64              `json:"id"`
	UserID    int64              `json:"user_id"`
	Token     string             `json:"token"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

type Room struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	OwnerID   string `json:"owner_id"`
	Name      string `json:"name"`
	Members   []byte `json:"members"`
	Settings  []byte `json:"settings"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
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

internal/sqlc/querier.go
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type Querier interface {
	BlockUser(ctx context.Context, arg BlockUserParams) error
	CleanupOldMessages(ctx context.Context) error
	CreateLoginAttempt(ctx context.Context, arg CreateLoginAttemptParams) error
	CreateRefreshToken(ctx context.Context, arg CreateRefreshTokenParams) error
	CreateTwoFaCode(ctx context.Context, arg CreateTwoFaCodeParams) (int64, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	CreateUserClient(ctx context.Context, arg CreateUserClientParams) error
	DeleteExpiredRefreshTokens(ctx context.Context) error
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteRoom(ctx context.Context, id string) error
	GetBlockedStatus(ctx context.Context, email string) (pgtype.Timestamptz, error)
	GetFailedLogAttempts(ctx context.Context, arg GetFailedLogAttemptsParams) (int64, error)
	GetMessage(ctx context.Context, id string) (GetMessageRow, error)
	GetRecentCodeRequests(ctx context.Context, arg GetRecentCodeRequestsParams) (int64, error)
	GetRecentFailedAttempts(ctx context.Context, arg GetRecentFailedAttemptsParams) (int64, error)
	GetRecentVerificationAttempts(ctx context.Context, arg GetRecentVerificationAttemptsParams) (int64, error)
	GetRefreshToken(ctx context.Context, token string) (GetRefreshTokenRow, error)
	GetRoomByID(ctx context.Context, id string) (Room, error)
	GetRoomMessages(ctx context.Context, arg GetRoomMessagesParams) ([]GetRoomMessagesRow, error)
	GetTwoFaCodeByUserID(ctx context.Context, userID int64) (TwoFaCode, error)
	GetUserByEmail(ctx context.Context, email string) (GetUserByEmailRow, error)
	GetUserByID(ctx context.Context, id int64) (GetUserByIDRow, error)
	ListUserRooms(ctx context.Context, dollar_1 []byte) ([]Room, error)
	MarkTwoFaCodeAsUsed(ctx context.Context, id int64) error
	RefreshDeleteByUserI(ctx context.Context, userID int64) error
	RefreshDeleteByUserID(ctx context.Context, userID int64) error
	ResetFailedAttempts(ctx context.Context, email string) error
	SaveMessage(ctx context.Context, arg SaveMessageParams) error
	UpdatePasswordHash(ctx context.Context, arg UpdatePasswordHashParams) error
	UpdateRoom(ctx context.Context, arg UpdateRoomParams) error
	UpdateTwoFAStatus(ctx context.Context, arg UpdateTwoFAStatusParams) error
	UpdateTwoFaCodeAttempts(ctx context.Context, arg UpdateTwoFaCodeAttemptsParams) error
	UpdateUserClient(ctx context.Context, arg UpdateUserClientParams) error
	UpdateUserLastActive(ctx context.Context, arg UpdateUserLastActiveParams) error
	UpsertRoom(ctx context.Context, arg UpsertRoomParams) error
}

var _ Querier = (*Queries)(nil)
```

internal/sqlc/messages.sql.qo
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1
// source: messages.sql

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const cleanupOldMessages = `-- name: CleanupOldMessages :exec
DELETE FROM messages WHERE expires_at < NOW()
`

func (q *Queries) CleanupOldMessages(ctx context.Context) error {
	_, err := q.db.Exec(ctx, cleanupOldMessages)
	return err
}

const getMessage = `-- name: GetMessage :one
SELECT id, type, user_id, room_id, timestamp, payload, signature, created_at
FROM messages
WHERE id = $1 LIMIT 1
`

type GetMessageRow struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	UserID    string             `json:"user_id"`
	RoomID    string             `json:"room_id"`
	Timestamp int64              `json:"timestamp"`
	Payload   []byte             `json:"payload"`
	Signature pgtype.Text        `json:"signature"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

func (q *Queries) GetMessage(ctx context.Context, id string) (GetMessageRow, error) {
	row := q.db.QueryRow(ctx, getMessage, id)
	var i GetMessageRow
	err := row.Scan(
		&i.ID,
		&i.Type,
		&i.UserID,
		&i.RoomID,
		&i.Timestamp,
		&i.Payload,
		&i.Signature,
		&i.CreatedAt,
	)
	return i, err
}

const getRoomMessages = `-- name: GetRoomMessages :many
SELECT id, type, user_id, room_id, timestamp, payload, signature, created_at
FROM messages
WHERE room_id = $1
ORDER BY timestamp DESC
LIMIT $2
`

type GetRoomMessagesParams struct {
	RoomID string `json:"room_id"`
	Limit  int32  `json:"limit"`
}

type GetRoomMessagesRow struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	UserID    string             `json:"user_id"`
	RoomID    string             `json:"room_id"`
	Timestamp int64              `json:"timestamp"`
	Payload   []byte             `json:"payload"`
	Signature pgtype.Text        `json:"signature"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

func (q *Queries) GetRoomMessages(ctx context.Context, arg GetRoomMessagesParams) ([]GetRoomMessagesRow, error) {
	rows, err := q.db.Query(ctx, getRoomMessages, arg.RoomID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []GetRoomMessagesRow{}
	for rows.Next() {
		var i GetRoomMessagesRow
		if err := rows.Scan(
			&i.ID,
			&i.Type,
			&i.UserID,
			&i.RoomID,
			&i.Timestamp,
			&i.Payload,
			&i.Signature,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const saveMessage = `-- name: SaveMessage :exec
INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days')
`

type SaveMessageParams struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	UserID    string      `json:"user_id"`
	RoomID    string      `json:"room_id"`
	Timestamp int64       `json:"timestamp"`
	Payload   []byte      `json:"payload"`
	Signature pgtype.Text `json:"signature"`
}

func (q *Queries) SaveMessage(ctx context.Context, arg SaveMessageParams) error {
	_, err := q.db.Exec(ctx, saveMessage,
		arg.ID,
		arg.Type,
		arg.UserID,
		arg.RoomID,
		arg.Timestamp,
		arg.Payload,
		arg.Signature,
	)
	return err
}
```

internal/sqlc/rooms.sql.go
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1
// source: rooms.sql

package database

import (
	"context"
)

const deleteRoom = `-- name: DeleteRoom :exec
DELETE FROM rooms WHERE id = $1
`

func (q *Queries) DeleteRoom(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, deleteRoom, id)
	return err
}

const getRoomByID = `-- name: GetRoomByID :one
SELECT id, type, owner_id, name, members, settings, created_at, updated_at
FROM rooms
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetRoomByID(ctx context.Context, id string) (Room, error) {
	row := q.db.QueryRow(ctx, getRoomByID, id)
	var i Room
	err := row.Scan(
		&i.ID,
		&i.Type,
		&i.OwnerID,
		&i.Name,
		&i.Members,
		&i.Settings,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const listUserRooms = `-- name: ListUserRooms :many
SELECT id, type, owner_id, name, members, settings, created_at, updated_at FROM rooms
WHERE members @> $1::jsonb
`

func (q *Queries) ListUserRooms(ctx context.Context, dollar_1 []byte) ([]Room, error) {
	rows, err := q.db.Query(ctx, listUserRooms, dollar_1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Room{}
	for rows.Next() {
		var i Room
		if err := rows.Scan(
			&i.ID,
			&i.Type,
			&i.OwnerID,
			&i.Name,
			&i.Members,
			&i.Settings,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateRoom = `-- name: UpdateRoom :exec
UPDATE rooms
SET name = $2, members = $3, settings = $4, updated_at = $5
WHERE id = $1
`

type UpdateRoomParams struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Members   []byte `json:"members"`
	Settings  []byte `json:"settings"`
	UpdatedAt int64  `json:"updated_at"`
}

func (q *Queries) UpdateRoom(ctx context.Context, arg UpdateRoomParams) error {
	_, err := q.db.Exec(ctx, updateRoom,
		arg.ID,
		arg.Name,
		arg.Members,
		arg.Settings,
		arg.UpdatedAt,
	)
	return err
}

const upsertRoom = `-- name: UpsertRoom :exec
INSERT INTO rooms (id, type, owner_id, name, members, settings, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    members = EXCLUDED.members,
    settings = EXCLUDED.settings,
    updated_at = EXCLUDED.updated_at
`

type UpsertRoomParams struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	OwnerID   string `json:"owner_id"`
	Name      string `json:"name"`
	Members   []byte `json:"members"`
	Settings  []byte `json:"settings"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (q *Queries) UpsertRoom(ctx context.Context, arg UpsertRoomParams) error {
	_, err := q.db.Exec(ctx, upsertRoom,
		arg.ID,
		arg.Type,
		arg.OwnerID,
		arg.Name,
		arg.Members,
		arg.Settings,
		arg.CreatedAt,
		arg.UpdatedAt,
	)
	return err
}
```

internal/sqlc/two_fa.sql.go
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1
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

internal/sqlc/user.sql.go
```
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1
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

const createUserClient = `-- name: CreateUserClient :exec
INSERT INTO users (id, username, firstname, lastname, email, password_hash)
VALUES ($1, $2, $3, $4, $5, $6)
`

type CreateUserClientParams struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	Firstname    string `json:"firstname"`
	Lastname     string `json:"lastname"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
}

func (q *Queries) CreateUserClient(ctx context.Context, arg CreateUserClientParams) error {
	_, err := q.db.Exec(ctx, createUserClient,
		arg.ID,
		arg.Username,
		arg.Firstname,
		arg.Lastname,
		arg.Email,
		arg.PasswordHash,
	)
	return err
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

const updateUserClient = `-- name: UpdateUserClient :exec
UPDATE users
SET username = $2, firstname = $3, lastname = $4, email = $5
WHERE id = $1
`

type UpdateUserClientParams struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Email     string `json:"email"`
}

func (q *Queries) UpdateUserClient(ctx context.Context, arg UpdateUserClientParams) error {
	_, err := q.db.Exec(ctx, updateUserClient,
		arg.ID,
		arg.Username,
		arg.Firstname,
		arg.Lastname,
		arg.Email,
	)
	return err
}

const updateUserLastActive = `-- name: UpdateUserLastActive :exec
UPDATE users
SET updated_at = $2
WHERE id = $1
`

type UpdateUserLastActiveParams struct {
	ID        int64              `json:"id"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}

func (q *Queries) UpdateUserLastActive(ctx context.Context, arg UpdateUserLastActiveParams) error {
	_, err := q.db.Exec(ctx, updateUserLastActive, arg.ID, arg.UpdatedAt)
	return err
}
```

pkg/apperror/error.go
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

pkg/apperror/middleware.go
```
package apperror

import (
	"fmt"
	"krampus/internal/user/domain"
	"krampus/internal/user/storage"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type appHandler func(w http.ResponseWriter, r *http.Request) error

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

// CORSMiddleware — разрешает кросс-доменные запросы
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

// AuthMiddleware — умная авторизация (Redis + JWT)
func AuthMiddleware(redisStorage storage.RedisSessionStorage, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Проверка формата Bearer
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Error(New(ErrUnauthorized, "invalid auth format (bearer required)"))
			c.Abort()
			return
		}

		tokenString := authHeader[7:]

		// 1. Пытаемся взять из кэша Redis
		if any(redisStorage) != nil { // Измените на это
			if session, err := redisStorage.GetAccessToken(tokenString); err == nil {
				c.Set("user_id", fmt.Sprintf("%d", session.UserID))
				c.Next()
				return
			}
		}

		// 2. Если в кэше нет — парсим JWT
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.Error(New(ErrUnauthorized, "token is invalid or expired"))
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.Error(New(ErrUnauthorized, "invalid token claims"))
			c.Abort()
			return
		}

		// Извлекаем userID (обычно float64 в JWT)
		userIDRaw, ok := claims["user_id"]
		if !ok {
			c.Error(New(ErrUnauthorized, "user_id not found in token"))
			c.Abort()
			return
		}

		userID := int64(userIDRaw.(float64))
		userIDStr := fmt.Sprintf("%d", userID)

		// 3. Кэшируем результат для следующих запросов
		if any(redisStorage) != nil { // И здесь тоже
			_ = redisStorage.SetAccessToken(tokenString, domain.CachedSession{
				UserID:    userID,
				ExpiresAt: time.Now().Add(15 * time.Minute),
			})
		}

		c.Set("user_id", userIDStr)
		c.Next()
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

pkg/auth/jwt.go
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

pkg/client-database/postgresql/postgres.go
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

pkg/client-database/redis/redis.go
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

pkg/config/config.go
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
	JWTSecret   string `env:"JWT_SECRET"`
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

pkg/hash/hash.go
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

pkg/logging/logging.go
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

pkg/messaging/kafka/consumer.go
```
package kafka

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"krampus/internal/message/domain"
	"krampus/pkg/config"
	"krampus/pkg/logging"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type Consumer struct {
	consumer *kafka.Consumer
	logger   logging.Logger
	topic    string
	handlers []func(*domain.BaseMessage)
	mu       sync.RWMutex
}

func NewConsumer(cfg config.KafkaConfig, logger logging.Logger) (*Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":     cfg.Brokers,
		"group.id":              "krampus-consumer-group",
		"auto.offset.reset":     "latest",
		"enable.auto.commit":    true,
		"session.timeout.ms":    6000,
		"heartbeat.interval.ms": 3000,
	})
	if err != nil {
		return nil, err
	}

	if err := c.SubscribeTopics([]string{cfg.Topics.Incoming}, nil); err != nil {
		return nil, err
	}

	return &Consumer{
		consumer: c,
		logger:   logger,
		topic:    cfg.Topics.Incoming,
	}, nil
}

func (c *Consumer) AddHandler(handler func(*domain.BaseMessage)) {
	c.mu.Lock()
	c.handlers = append(c.handlers, handler)
	c.mu.Unlock()
}

func (c *Consumer) Consume(ctx context.Context) {
	defer c.consumer.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := c.consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				continue
			}

			var event struct {
				ID        string          `json:"id"`
				Type      string          `json:"type"`
				UserID    string          `json:"user_id"`
				RoomID    string          `json:"room_id"`
				Timestamp int64           `json:"timestamp"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				c.logger.Errorf("Failed to unmarshal Kafka msg: %v", err)
				continue
			}

			msgObj := &domain.BaseMessage{
				ID:        event.ID,
				Type:      domain.MessageType(event.Type),
				UserID:    event.UserID,
				RoomID:    event.RoomID,
				Timestamp: event.Timestamp,
				Payload:   event.Payload,
			}

			c.mu.RLock()
			for _, handler := range c.handlers {
				go handler(msgObj)
			}
			c.mu.RUnlock()
		}
	}
}
```

pkg/messaging/kafka/producer.go
```
package kafka

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"krampus/internal/message/domain"
	"krampus/pkg/config"
	"krampus/pkg/logging"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type Producer struct {
	producer *kafka.Producer
	logger   logging.Logger
	topic    string
	wg       sync.WaitGroup
}

func NewProducer(cfg config.KafkaConfig, logger logging.Logger) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(cfg.Brokers, ","),
		"client.id":          "krampus-producer",
		"acks":               "all",
		"retries":            5,
		"enable.idempotence": true,
		"linger.ms":          10,
		"batch.size":         16384,
	})
	if err != nil {
		return nil, err
	}

	producer := &Producer{
		producer: p,
		logger:   logger,
		topic:    cfg.Topics.Incoming,
	}
	producer.wg.Add(1)
	go producer.deliveryReport()
	return producer, nil
}

func (p *Producer) Publish(ctx context.Context, msg *domain.BaseMessage) error {
	// data, err := json.Marshal(msg)
	// if err != nil {
	// 	return err
	// }

	event := struct {
		ID        string          `json:"id"`
		Type      string          `json:"type"`
		UserID    string          `json:"user_id"`
		RoomID    string          `json:"room_id"`
		Timestamp int64           `json:"timestamp"`
		Payload   json.RawMessage `json:"payload"`
	}{
		ID:        msg.ID,
		Type:      string(msg.Type),
		UserID:    msg.UserID,
		RoomID:    msg.RoomID,
		Timestamp: msg.Timestamp,
		Payload:   msg.Payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &p.topic, Partition: kafka.PartitionAny},
		Key:            []byte(msg.RoomID),
		Value:          data,
		Headers:        []kafka.Header{{Key: "source", Value: []byte("krampus-api")}},
	}, nil)

	return err
}

func (p *Producer) deliveryReport() {
	defer p.wg.Done()
	for e := range p.producer.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				p.logger.Errorf("Delivery failed: %v", ev.TopicPartition.Error)
			} else {
				p.logger.Infof("Delivered to %s [%d] at offset %v", *ev.TopicPartition.Topic, ev.TopicPartition, ev.TopicPartition.Offset)
			}
		}
	}
}

func (p *Producer) Close() {
	p.producer.Flush(15 * 1000)
	p.producer.Close()
	p.wg.Wait()
}
```

pkg/server/gin.go
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

pkg/utils/random.go
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

pkg/utils/repeatable.go
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

pkg/validation/validation.go
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

sql/queries/two_fa.sql
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

sql/queries/users.sql
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

-- name: UpdateUserLastActive :exec
UPDATE users
SET updated_at = $2
WHERE id = $1;

-- name: CreateUserClient :exec
INSERT INTO users (id, username, firstname, lastname, email, password_hash)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateUserClient :exec
UPDATE users
SET username = $2, firstname = $3, lastname = $4, email = $5
WHERE id = $1;
```

sql/queries/messages.sql
```
-- name: SaveMessage :exec
INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days');

-- name: GetMessage :one
SELECT id, type, user_id, room_id, timestamp, payload, signature, created_at
FROM messages
WHERE id = $1 LIMIT 1;

-- name: GetRoomMessages :many
SELECT id, type, user_id, room_id, timestamp, payload, signature, created_at
FROM messages
WHERE room_id = $1
ORDER BY timestamp DESC
LIMIT $2;

-- name: CleanupOldMessages :exec
DELETE FROM messages WHERE expires_at < NOW();
```

sql/queries/rooms.sql
```
-- name: GetRoomByID :one
SELECT id, type, owner_id, name, members, settings, created_at, updated_at
FROM rooms
WHERE id = $1 LIMIT 1;

-- name: UpsertRoom :exec
INSERT INTO rooms (id, type, owner_id, name, members, settings, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    members = EXCLUDED.members,
    settings = EXCLUDED.settings,
    updated_at = EXCLUDED.updated_at;

-- name: UpdateRoom :exec
UPDATE rooms
SET name = $2, members = $3, settings = $4, updated_at = $5
WHERE id = $1;

-- name: DeleteRoom :exec
DELETE FROM rooms WHERE id = $1;

-- name: ListUserRooms :many
SELECT * FROM rooms
WHERE members @> $1::jsonb;
```

sql/schema/init.sql
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

CREATE TABLE IF NOT EXISTS rooms (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL,
    owner_id   TEXT NOT NULL,
    name       TEXT NOT NULL,
    members    JSONB NOT NULL, -- список ID участников
    settings   JSONB NOT NULL, -- объект настроек
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    room_id     TEXT NOT NULL,
    timestamp   BIGINT NOT NULL,
    payload     JSONB NOT NULL,
    signature   TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at  TIMESTAMP WITH TIME ZONE
);

-- Индексы (sqlc их не генерирует, но они нужны в БД)
CREATE INDEX IF NOT EXISTS idx_messages_room_timestamp ON messages(room_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_messages_expires ON messages(expires_at) WHERE expires_at IS NOT NULL;
```
