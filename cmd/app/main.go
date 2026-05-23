package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	authRest "krampus/internal/auth/adapters"
	authService "krampus/internal/auth/service"
	authStorage "krampus/internal/auth/storage"

	audit "krampus/internal/audit"

	chatAdapters "krampus/internal/chat/adapters"
	chatService "krampus/internal/chat/service"
	chatStorage "krampus/internal/chat/storage"

	"krampus/internal/events"

	identityService "krampus/internal/identity/service"

	messageAdapters "krampus/internal/message/adapters"
	messageService "krampus/internal/message/service"
	messageStorage "krampus/internal/message/storage"

	"krampus/internal/moderation"
	"krampus/internal/polls"
	"krampus/internal/reactions"
	"krampus/internal/retention"
	"krampus/internal/search"
	"krampus/internal/stickers"
	"krampus/internal/sync"

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
)

func main() {
	logging.Init()

	logger := logging.GetLogger()
	logger.Infoln("🚀 Starting Krampus")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("config load failed: %v", err)
	}

	overrideConfigFromEnv(cfg)

	gin.SetMode(cfg.Env)

	// -------------------------------------------------------------------------
	// DATABASES
	// -------------------------------------------------------------------------

	pool, err := postgresql.NewClient(ctx, 15, *cfg)
	if err != nil {
		logger.Fatalf("postgres init failed: %v", err)
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	redisWrapper, err := redisClient.New(redisClient.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: 10,
	})
	if err != nil {
		logger.Fatalf("redis init failed: %v", err)
	}

	sqlDB, err := sql.Open(
		"postgres",
		cfg.PostgresDSN,
	)

	// -------------------------------------------------------------------------
	// KAFKA
	// -------------------------------------------------------------------------

	kafkaCfg := config.KafkaConfig{
		Brokers: cfg.Kafka.Brokers,
		Topics:  cfg.Kafka.Topics,
	}

	producer := messageStorage.NewMessageDistributor(kafkaCfg, *logger)

	kafkaConsumer, err := kafka.NewConsumer(kafkaCfg, *logger)
	if err != nil {
		logger.Fatalf("kafka consumer init failed: %v", err)
	}

	// -------------------------------------------------------------------------
	// SECURITY
	// -------------------------------------------------------------------------

	jwtSecret := getJWTSecret()

	// -------------------------------------------------------------------------
	// USER MODULE
	// -------------------------------------------------------------------------

	loginAttemptStorage := userStorage.NewLoginAttemptStorage(queries)
	userPG := userStorage.NewUserStorage(queries)
	sessionRedis := userStorage.NewRedisSessionStorage(redisWrapper.RDB())
	refreshStorage := userStorage.NewRefreshTokenStorage(queries)

	twoFAStorage := authStorage.NewStorage(queries)

	refreshSvc := userService.NewRefreshToken(refreshStorage, jwtSecret)

	userSvc := userService.NewUser(
		userPG,
		loginAttemptStorage,
		refreshSvc,
		sessionRedis,
		jwtSecret,
	)

	twoFASvc := authService.NewTwoFA(
		twoFAStorage,
		refreshSvc,
		sessionRedis,
		jwtSecret,
	)

	userHandler := userRest.NewUserHandler(userSvc, logger)
	refreshHandler := userRest.NewRefreshTokenHandler(refreshSvc, logger)
	twoFAHandler := authRest.NewTwoFAHandler(twoFASvc, logger)

	// -------------------------------------------------------------------------
	// CHAT MODULE
	// -------------------------------------------------------------------------

	userClientRedis := chatStorage.NewUserClientCache(redisWrapper)
	userClientStorage := chatStorage.NewUserClientPGStorage(queries)

	roomRedis := chatStorage.NewRoomCache(redisWrapper)
	roomStorage := chatStorage.NewRoomPGStorage(queries)

	userClientSvc := chatService.NewUserClientService(
		userClientStorage,
		userClientRedis,
	)

	roomSvc := chatService.NewRoomService(
		roomStorage,
		roomRedis,
		userClientSvc,
	)

	// -------------------------------------------------------------------------
	// MESSAGE MODULE
	// -------------------------------------------------------------------------

	fileStorage := messageStorage.NewFileStorage(
		cfg.File.BasePath,
		cfg.File.SegmentSize,
		roomSvc,
	)

	_ = fileStorage

	messagePG := messageStorage.NewMessagePGStorage(pool, queries)

	messageSvc := messageService.NewMessageService(
		messagePG,
		producer,
		roomSvc,
		userClientSvc,
	)

	// -------------------------------------------------------------------------
	// IDENTITY + WS AUTH
	// -------------------------------------------------------------------------

	jwtSvc := identityService.NewJWTService(jwtSecret)

	wsAuthSvc := identityService.NewWSAuthService(
		jwtSvc,
		sessionRedis,
		roomSvc,
	)

	// -------------------------------------------------------------------------
	// OPTIONAL / BACKGROUND MODULES
	// -------------------------------------------------------------------------

	// moderation
	moderationRepo := moderation.NewRepository(sqlDB)
	moderationTools := moderation.NewTools()
	moderationProjection := moderation.NewProjection(moderationRepo)

	_ = moderationTools
	_ = moderationProjection

	// polls
	pollService := polls.NewService(sqlDB)
	pollProjection := polls.NewProjection(sqlDB)
	pollClosingWorker := polls.NewClosingWorker(pollService)

	_ = pollProjection
	go pollClosingWorker.Start(ctx)

	// reactions
	reactionService := reactions.NewService(queries)
	_ = reactionService

	// stickers
	stickerService := stickers.NewService()
	_ = stickerService

	// retention
	retentionRepo := retention.NewRepository(queries)
	retentionExecutor := retention.NewExecutor(retentionRepo)
	retentionWorker := retention.NewWorker(retentionExecutor)

	go retentionWorker.Start(ctx)

	// sync
	syncService := sync.NewService(redisWrapper.RDB())
	_ = syncService

	// notifications
	// fcmProvider := notificationProviders.NewFCMProvider()
	// notificationSvc := notificationService.New(fcmProvider)

	// search
	searchConsumer := search.NewConsumer(kafkaConsumer, queries)
	go searchConsumer.Start(ctx)

	// audit
	auditConsumer := audit.NewConsumer(kafkaConsumer, queries)
	go auditConsumer.Start(ctx)

	// events
	eventProjection := events.NewProjection(queries)
	_ = eventProjection

	// -------------------------------------------------------------------------
	// ROUTERS + WEBSOCKET
	// -------------------------------------------------------------------------

	chatRouter := chatAdapters.NewRouter(
		sessionRedis,
		roomSvc,
		messageSvc,
		userClientSvc,
		cfg,
	)

	wsServer := messageAdapters.NewWebSocketServer(
		messageSvc,
		cfg,
		wsAuthSvc,
		logger,
	)

	// -------------------------------------------------------------------------
	// HTTP SERVER
	// -------------------------------------------------------------------------

	engine := gin.New()

	engine.Use(
		gin.Recovery(),
		apperror.CORSMiddleware(),
		apperror.ErrorMiddleware(),
	)

	engine.GET("/health", healthHandler)

	api := engine.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		{
			userHandler.RegisterRoutes(authGroup)
			refreshHandler.RegisterRoutes(authGroup)
			twoFAHandler.RegisterRoutes(authGroup)
		}

		chatGroup := api.Group("/chat")
		chatGroup.Use(
			apperror.AuthMiddleware(*sessionRedis, jwtSecret),
		)
		{
			chatRouter.RegisterRoutes(chatGroup)
		}
	}

	engine.GET("/ws", func(c *gin.Context) {
		wsServer.HandleWebSocket(c.Writer, c.Request)
	})

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           engine,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Infof("HTTP server started on :%s", cfg.HTTPPort)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server failed: %v", err)
		}
	}()

	// -------------------------------------------------------------------------
	// GRACEFUL SHUTDOWN
	// -------------------------------------------------------------------------

	quit := make(chan os.Signal, 1)

	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	<-quit

	logger.Warnln("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("shutdown failed: %v", err)
	}

	logger.Infoln("server stopped")
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
	})
}

func overrideConfigFromEnv(cfg *config.Config) {
	if env := os.Getenv("APP_ENV"); env != "" {
		cfg.Env = env
	}

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
