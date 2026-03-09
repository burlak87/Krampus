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
