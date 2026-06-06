package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	// Registers the "pgx" database/sql driver used by sql.Open below.
	_ "github.com/jackc/pgx/v5/stdlib"

	callAdapters "krampus/internal/call/adapters"
	callService "krampus/internal/call/service"
	callStorage "krampus/internal/call/storage"

	authRest "krampus/internal/auth/adapters"
	authService "krampus/internal/auth/service"
	authStorage "krampus/internal/auth/storage"

	auditSvc "krampus/internal/audit/service"

	chatAdapters "krampus/internal/chat/adapters"
	filesService "krampus/internal/files/service"
	filesWorkers "krampus/internal/files/workers"
	filesStorage "krampus/internal/files/storage"
	mediaStorage "krampus/internal/media/storage"
	mediaSvc "krampus/internal/media/service"
	"krampus/internal/profile/avatar"
	chatService "krampus/internal/chat/service"
	chatStorage "krampus/internal/chat/storage"

	eventsSvc "krampus/internal/events/service"

	identityService "krampus/internal/identity/service"

	messageAdapters "krampus/internal/message/adapters"
	messageService "krampus/internal/message/service"
	messageStorage "krampus/internal/message/storage"

	moderationService "krampus/internal/moderation/service"
	moderationStorage "krampus/internal/moderation/storage"
	notifProviders "krampus/internal/notifications/providers"
	notifService "krampus/internal/notifications/service"
	"krampus/internal/permissions"
	pollsService "krampus/internal/polls/service"
	pollsWorkers "krampus/internal/polls/workers"
	reactionsService "krampus/internal/reactions/service"
	retentionSvc "krampus/internal/retention/service"
	retentionStorage "krampus/internal/retention/storage"
	retentionWorkers "krampus/internal/retention/workers"
	searchSvc "krampus/internal/search/service"
	stickersService "krampus/internal/stickers/service"
	syncSvc "krampus/internal/sync/service"

	supervisor "krampus/internal/platform/supervisor"

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

	// Map the app environment to a valid Gin mode (Gin only accepts
	// debug/release/test, not "development"/"production").
	switch cfg.Env {
	case "production", "prod", gin.ReleaseMode:
		gin.SetMode(gin.ReleaseMode)
	case gin.TestMode:
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

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

	sqlDB, err := sql.Open("pgx", cfg.PostgresDSN)
	if err != nil {
		logger.Fatalf("sql.DB open failed: %v", err)
	}

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

	messagePG := messageStorage.NewMessagePGStorage(pool, queries)

	outboxRepo := messageStorage.NewOutboxRepositoryPsql(pool)
	idempotencyRepo := messageStorage.NewIdempotencyRepositoryPsql(pool)
	retryRepo := messageStorage.NewPSQLRetryRepository(queries)
	dlqRepo := messageStorage.NewPSQLDLQRepository(queries)
	deliveryRepo := messageStorage.NewPSQLDeliveryStatusRepository(queries)
	replayRepo := messageStorage.NewPSQLReplayRepository(queries)

	messageSvc := messageService.NewMessageService(
		messagePG,
		producer,
		roomSvc,
		userClientSvc,
		outboxRepo,
		idempotencyRepo,
	)

	// Wire file-backed secondary store (Phase 10)
	messageSvc.SetFileStore(fileStorage)

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
	// EVENT INFRASTRUCTURE
	// -------------------------------------------------------------------------

	eventBus := eventsSvc.NewBus()
	checkpointRepo := eventsSvc.NewCheckpointRepository(sqlDB)
	ownership := eventsSvc.NewOwnership(sqlDB)
	eventCoordinator := eventsSvc.NewCoordinator(ownership, cfg.NodeID, 4)

	// The event-sourcing coordinator continuously acquires partition leases in
	// event_partition_leases. That table isn't in the base schema yet, so the
	// worker is opt-in to avoid spamming logs with "relation does not exist".
	// Set EVENTS_ENABLED=true once the event-sourcing schema is provisioned.
	if os.Getenv("EVENTS_ENABLED") == "true" {
		go supervisor.RunWorker(ctx, "event-coordinator", eventCoordinator.Start)
	} else {
		logger.Infoln("event-coordinator disabled (set EVENTS_ENABLED=true to enable)")
	}

	// -------------------------------------------------------------------------
	// AUDIT CONSUMER
	// -------------------------------------------------------------------------

	auditConsumer := auditSvc.NewConsumer(sqlDB)
	eventBus.Subscribe("message_created", auditConsumer)
	eventBus.Subscribe("moderation_action_created", auditConsumer)
	eventBus.Subscribe("poll_vote_cast", auditConsumer)
	eventBus.Subscribe("reaction_added", auditConsumer)

	auditProjector := eventsSvc.NewProjector([]eventsSvc.Projection{auditConsumer})
	auditEventConsumer := eventsSvc.NewConsumer(sqlDB, auditProjector, checkpointRepo, "audit", 100)
	go supervisor.RunWorker(ctx, "audit-consumer", auditEventConsumer.Start)

	// -------------------------------------------------------------------------
	// SEARCH CONSUMER
	// -------------------------------------------------------------------------

	searchIndexer := searchSvc.NewIndexer(sqlDB)
	searchConsumer := searchSvc.NewConsumer(searchIndexer)

	eventBus.Subscribe("message_created", searchConsumer)

	searchProjector := eventsSvc.NewProjector([]eventsSvc.Projection{searchConsumer})
	searchEventConsumer := eventsSvc.NewConsumer(sqlDB, searchProjector, checkpointRepo, "search", 100)
	go supervisor.RunWorker(ctx, "search-consumer", searchEventConsumer.Start)

	// -------------------------------------------------------------------------
	// MODERATION
	// -------------------------------------------------------------------------

	moderationRepo := moderationStorage.NewRepository(sqlDB)
	_ = moderationRepo
	moderationTools := moderationService.NewTools(sqlDB)
	shadowRepo := moderationStorage.NewShadowRepository(sqlDB)
	deliverySuppressor := moderationService.NewDeliverySuppressor(shadowRepo)
	moderationProjection := &moderationService.Projection{}

	eventBus.Subscribe("moderation_action_created", moderationProjection)

	// -------------------------------------------------------------------------
	// PERMISSIONS
	// -------------------------------------------------------------------------

	permissionsRepo := permissions.NewPostgresRepository(sqlDB)
	permissionsSvc := permissions.NewService(permissionsRepo)

	// -------------------------------------------------------------------------
	// POLLS
	// -------------------------------------------------------------------------

	pollsSvc := pollsService.NewService(sqlDB)
	pollProjection := pollsService.NewProjection(sqlDB)
	pollClosingWorker := pollsWorkers.NewClosingWorker(sqlDB)

	go supervisor.RunWorker(ctx, "poll-closing-worker", pollClosingWorker.Start)

	// -------------------------------------------------------------------------
	// REACTIONS / STICKERS
	// -------------------------------------------------------------------------

	reactionService := reactionsService.NewService(sqlDB)
	stickerService := stickersService.NewService(sqlDB)

	// -------------------------------------------------------------------------
	// RETENTION
	// -------------------------------------------------------------------------

	retentionRepo := retentionStorage.NewRepository(sqlDB)
	retentionExecutor := retentionSvc.NewExecutor(sqlDB)
	retentionWorker := retentionWorkers.NewWorker(sqlDB)

	go supervisor.RunWorker(ctx, "retention-worker", retentionWorker.Start)

	go supervisor.RunWorker(ctx, "retention-policy-executor", func(workerCtx context.Context) {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				policies, err := retentionRepo.GetPolicies(workerCtx)
				if err != nil {
					logger.Errorf("retention policy fetch failed: %v", err)
					continue
				}
				for _, p := range policies {
					if err := retentionExecutor.Execute(workerCtx, p); err != nil {
						logger.Errorf("retention execute policy=%s err=%v", p.MediaType, err)
					}
				}
			}
		}
	})

	// -------------------------------------------------------------------------
	// SYNC SERVICE
	// -------------------------------------------------------------------------

	syncService := syncSvc.NewService(sqlDB)

	// -------------------------------------------------------------------------
	// MEDIA / AVATAR
	// -------------------------------------------------------------------------

	s3Client, err := filesStorage.NewS3Client(ctx, cfg)
	if err != nil {
		logger.Fatalf("s3 client init failed: %v", err)
	}

	s3Store := filesStorage.NewS3Storage(s3Client, cfg.S3.Bucket)
	mediaRepo := mediaStorage.New(sqlDB)
	mediaService := mediaSvc.New(s3Store, mediaRepo, nil, nil)

	avatarRepo := avatar.NewRepository(sqlDB)
	avatarSvc := avatar.New(mediaService, avatarRepo)

	// -------------------------------------------------------------------------
	// PUSH NOTIFICATIONS
	// -------------------------------------------------------------------------

	fcmProvider := notifProviders.NewFCMProvider()
	notificationSvc := notifService.New(fcmProvider)
	_ = notificationSvc

	// -------------------------------------------------------------------------
	// UPLOAD BACKGROUND WORKERS
	// -------------------------------------------------------------------------

	uploadRepo := filesService.NewRepository(sqlDB)
	objectStore := filesStorage.NewS3Storage(s3Client, cfg.S3.Bucket)
	integrityVerifier := filesService.NewIntegrityVerifier(objectStore)
	resumeVerifier := filesService.NewResumeVerifier(objectStore)

	go supervisor.RunWorker(ctx, "upload-cleanup", filesWorkers.NewCleanupWorker(sqlDB).Start)
	go supervisor.RunWorker(ctx, "upload-integrity", filesWorkers.NewIntegrityWorker(uploadRepo, integrityVerifier).Start)
	go supervisor.RunWorker(ctx, "upload-repair", filesWorkers.NewRepairWorker(uploadRepo, resumeVerifier).Start)

	// -------------------------------------------------------------------------
	// ROUTERS + WEBSOCKET
	// -------------------------------------------------------------------------

	chatRouter := chatAdapters.NewRouter(
		sessionRedis,
		roomSvc,
		userClientSvc,
		messageSvc,
		cfg,
		pollsSvc,
		pollProjection,
		reactionService,
		stickerService,
		searchIndexer,
		syncService,
		moderationTools,
		permissionsSvc,
		avatarSvc,
	)

	wsServer := messageAdapters.NewWebSocketServer(
		messageSvc,
		cfg,
		kafkaConsumer,
		wsAuthSvc,
		retryRepo,
		dlqRepo,
		deliveryRepo,
		replayRepo,
	)

	// Install shadow-ban suppressor on the shared connection manager.
	wsServer.Manager().SetSuppressor(deliverySuppressor.AllowBroadcast)

	// SSE server shares the same ConnectionManager.
	sseServer := messageAdapters.NewSSEServer(
		wsAuthSvc,
		wsServer.Manager(),
		replayRepo,
	)

	// -------------------------------------------------------------------------
	// CALL / WEBRTC SIGNALING SERVER
	// -------------------------------------------------------------------------

	callPresence := callStorage.NewRedisPresence(
		redisWrapper.RDB(),
		cfg.Call.PresenceTTL,
	)

	crossNode := callService.NewRedisCrossNode(redisWrapper.RDB(), cfg.NodeID)
	callHub := callService.NewHub(callPresence, crossNode, cfg.Call.MaxPeers)
	go crossNode.Run(ctx, callHub)

	callWSServer := callAdapters.NewCallWSServer(callHub, wsAuthSvc)

	iceHandler := callAdapters.NewICEHandler(callAdapters.ICEConfig{
		StunServers: cfg.Call.StunServers,
		TurnURL:     cfg.Call.TurnURL,
		TurnUser:    cfg.Call.TurnUser,
		TurnPass:    cfg.Call.TurnPass,
	})

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
			chatGroup.GET("/call/ice-servers", iceHandler.Handle)
		}
	}

	engine.GET("/ws", func(c *gin.Context) {
		wsServer.HandleWebSocket(c.Writer, c.Request)
	})

	engine.GET("/sse", func(c *gin.Context) {
		sseServer.HandleSSE(c.Writer, c.Request)
	})

	engine.GET("/call/ws", func(c *gin.Context) {
		callWSServer.HandleCallWebSocket(c.Writer, c.Request)
	})

	// HTTP_PORT may arrive as ":8080" or "8080"; normalize to a single leading
	// colon so we don't produce "::8080" ("too many colons in address").
	addr := cfg.HTTPPort
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Infof("HTTP server started on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server failed: %v", err)
		}
	}()

	// -------------------------------------------------------------------------
	// GRACEFUL SHUTDOWN
	// -------------------------------------------------------------------------

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Warnln("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
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

