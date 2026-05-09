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
	identityService "krampus/internal/identity/service"
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

	// WS AUTH

	jwtSvc := identityService.NewJWTService(jwtSecret)

	wsAuthSvc := identityService.NewWSAuthService(jwtSvc, userRedis, roomSvc)

	// 7. HANDLERS
	chatRouter := chatAdapters.NewRouter(userRedis, roomSvc, msgSvc, userClientSvc, cfg)
	wsServer := messageAdapters.NewWebSocketServer(msgSvc, cfg, wsAuthSvc, logger)

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
