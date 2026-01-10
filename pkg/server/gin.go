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