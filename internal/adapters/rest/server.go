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
