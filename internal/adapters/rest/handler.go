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
