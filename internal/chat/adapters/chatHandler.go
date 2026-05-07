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
