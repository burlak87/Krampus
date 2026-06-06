package adapters

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"krampus/internal/chat/domain"
	"krampus/internal/invites"
	messageDomain "krampus/internal/message/domain"
	messageService "krampus/internal/message/service"
	moderationSvc "krampus/internal/moderation/service"
	"krampus/internal/permissions"
	pollsSvc "krampus/internal/polls/service"
	"krampus/internal/profile/avatar"
	reactionsSvc "krampus/internal/reactions/service"
	searchSvc "krampus/internal/search/service"
	stickersSvc "krampus/internal/stickers/service"
	syncsvc "krampus/internal/sync/service"
	userDomain "krampus/internal/user/domain"
	redisStorage "krampus/internal/user/storage"
	"krampus/pkg/apperror"
	"krampus/pkg/config"
	"krampus/pkg/types"
)

type Room interface {
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	IsRoomMember(ctx context.Context, roomID types.RoomID, userID types.UserID) (bool, error)
	CanSendMessage(ctx context.Context, room *domain.Room, user *userDomain.User, msgType messageDomain.MessageType) bool
	CreateRoom(ctx context.Context, room *domain.Room) error
	UpdateRoom(ctx context.Context, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
	ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error)
}

type UserClient interface {
	GetUser(ctx context.Context, id string) (*userDomain.User, error)
	ListUsers(ctx context.Context, limit, offset int) ([]*userDomain.User, error)
	UpdateLastActivity(ctx context.Context, userID string)
	ValidateUserPermissions(ctx context.Context, userID string, required []string) error
	GetUserStatus(userID string) userDomain.ChatUserStatus
	SaveUser(ctx context.Context, user *userDomain.User) error
	UpdateUser(ctx context.Context, user *userDomain.User) error
}

type Router struct {
	RoomService        Room
	UserClientService  UserClient
	MessageService     *messageService.MessageService
	PollsService       *pollsSvc.Service
	PollProjection     *pollsSvc.Projection
	ReactionService    *reactionsSvc.Service
	StickerService     *stickersSvc.Service
	SearchIndexer      *searchSvc.Indexer
	SyncService        *syncsvc.Service
	ModerationTools    *moderationSvc.Tools
	PermissionsService *permissions.Service
	AvatarService      *avatar.Service
	config             config.Config
}

func NewRouter(
	_ *redisStorage.RedisSessionStorage,
	roomSvc Room,
	userSvc UserClient,
	msgSvc *messageService.MessageService,
	cfg *config.Config,
	polls *pollsSvc.Service,
	pollProj *pollsSvc.Projection,
	reactionSvc *reactionsSvc.Service,
	stickerSvc *stickersSvc.Service,
	searchIdx *searchSvc.Indexer,
	syncSvc *syncsvc.Service,
	modTools *moderationSvc.Tools,
	permSvc *permissions.Service,
	avatarSvc *avatar.Service,
) *Router {
	return &Router{
		RoomService:        roomSvc,
		UserClientService:  userSvc,
		MessageService:     msgSvc,
		PollsService:       polls,
		PollProjection:     pollProj,
		ReactionService:    reactionSvc,
		StickerService:     stickerSvc,
		SearchIndexer:      searchIdx,
		SyncService:        syncSvc,
		ModerationTools:    modTools,
		PermissionsService: permSvc,
		AvatarService:      avatarSvc,
		config:             *cfg,
	}
}

func (r *Router) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/messages", r.handleSendMessage())
	rg.GET("/rooms/:room_id/messages", r.handleGetMessages())
	rg.GET("/rooms/:room_id/search", r.handleSearchMessages())

	rg.POST("/rooms", r.handleCreateRoom())
	rg.GET("/rooms", r.handleListRooms())
	rg.GET("/rooms/:room_id", r.handleGetRoom())
	rg.POST("/rooms/join", r.handleJoinRoom())

	rg.GET("/users", r.handleListUsers())
	rg.GET("/users/:user_id", r.handleGetUser())

	rg.POST("/rooms/:room_id/polls/:poll_id/vote", r.handleVotePoll())
	rg.GET("/rooms/:room_id/polls/:poll_id", r.handleGetPollResults())

	rg.POST("/messages/:message_id/reactions", r.handleAddReaction())

	rg.GET("/stickers", r.handleListStickers())

	rg.GET("/sync", r.handleSync())

	rg.POST("/moderation/ban", r.handleBanUser())
	rg.POST("/moderation/mute", r.handleMuteUser())

	if r.AvatarService != nil {
		rg.POST("/profile/avatar", r.handleUploadAvatar())
		rg.GET("/profile/:user_id", r.handleGetProfile())
	}
}

func (r *Router) handleSendMessage() gin.HandlerFunc {
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
		msg.UserID = types.UserID(userID.(string))

		if err := r.MessageService.Process(c.Request.Context(), &msg); err != nil {
			c.Error(err.(*apperror.AppError))
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "sent", "msg_id": msg.ID})
	}
}

func (r *Router) handleGetMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("room_id")
		limitStr := c.DefaultQuery("limit", "50")
		limit, _ := strconv.Atoi(limitStr)
		if limit < 1 || limit > 1000 {
			limit = 50
		}

		msgs, err := r.MessageService.GetRoomMessages(c.Request.Context(), roomID, limit)
		if err != nil {
			c.Error(err.(*apperror.AppError))
			return
		}
		// Return a bare array, matching docs/API.md and the frontend client.
		if msgs == nil {
			msgs = []*messageDomain.BaseMessage{}
		}
		c.JSON(http.StatusOK, msgs)
	}
}

func (r *Router) handleSearchMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("room_id")
		q := c.Query("q")
		if q == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "q required"})
			return
		}
		limitStr := c.DefaultQuery("limit", "20")
		limit, _ := strconv.Atoi(limitStr)
		if limit < 1 || limit > 200 {
			limit = 20
		}

		results, err := r.SearchIndexer.Search(c.Request.Context(), roomID, q, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"results": results})
	}
}

func (r *Router) handleCreateRoom() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")

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

		if err := r.RoomService.CreateRoom(c.Request.Context(), &room); err != nil {
			c.Error(err.(*apperror.AppError))
			return
		}
		c.JSON(http.StatusCreated, room)
	}
}

func (r *Router) handleListRooms() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Error(apperror.New(apperror.ErrUnauthorized, "no auth"))
			return
		}
		rooms, err := r.RoomService.ListUserRooms(c.Request.Context(), userID.(string))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rooms"})
			return
		}
		c.JSON(http.StatusOK, rooms)
	}
}

func (r *Router) handleGetRoom() gin.HandlerFunc {
	return func(c *gin.Context) {
		room, err := r.RoomService.GetRoom(c.Request.Context(), c.Param("room_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusOK, room)
	}
}

func (r *Router) handleJoinRoom() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Error(apperror.New(apperror.ErrUnauthorized, "no auth"))
			return
		}

		var body struct {
			Token string `json:"token"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
			return
		}

		// Accept either a krampus://join/<id> deep link or a bare room id.
		roomID := body.Token
		if extracted, err := invites.ExtractJoinToken(body.Token); err == nil {
			roomID = extracted
		}

		room, err := r.RoomService.GetRoom(c.Request.Context(), roomID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
			return
		}

		// Grant membership so the joiner can connect to /ws and /call/ws
		// (WSAuthService enforces room membership).
		uid := userID.(string)
		isMember, _ := r.RoomService.IsRoomMember(c.Request.Context(), types.RoomID(roomID), types.UserID(uid))
		if !isMember {
			room.Members = append(room.Members, uid)
			if err := r.RoomService.UpdateRoom(c.Request.Context(), room); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "joined", "room": room})
	}
}

func (r *Router) handleListUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
		if limit < 1 || limit > 500 {
			limit = 100
		}
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		if offset < 0 {
			offset = 0
		}

		users, err := r.UserClientService.ListUsers(c.Request.Context(), limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
			return
		}
		c.JSON(http.StatusOK, users)
	}
}

func (r *Router) handleGetUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := r.UserClientService.GetUser(c.Request.Context(), c.Param("user_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

func (r *Router) handleVotePoll() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		pollID := c.Param("poll_id")

		var body struct {
			OptionID string `json:"option_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := r.PollsService.Vote(c.Request.Context(), pollID, body.OptionID, userID.(string)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "vote failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "voted"})
	}
}

func (r *Router) handleGetPollResults() gin.HandlerFunc {
	return func(c *gin.Context) {
		pollID := c.Param("poll_id")
		optionID := c.Query("option_id")
		if optionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "option_id required"})
			return
		}
		_ = pollID
		c.JSON(http.StatusOK, gin.H{"poll_id": pollID, "option_id": optionID})
	}
}

func (r *Router) handleAddReaction() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		messageID := c.Param("message_id")

		var body struct {
			Reaction string `json:"reaction" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := r.ReactionService.AddReaction(c.Request.Context(), messageID, userID.(string), body.Reaction); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "reaction failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func (r *Router) handleListStickers() gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = r.StickerService
		c.JSON(http.StatusOK, gin.H{"stickers": []any{}})
	}
}

func (r *Router) handleSync() gin.HandlerFunc {
	return func(c *gin.Context) {
		lastIDStr := c.DefaultQuery("last_event_id", "0")
		lastID, _ := strconv.ParseInt(lastIDStr, 10, 64)
		limitStr := c.DefaultQuery("limit", "100")
		limit, _ := strconv.Atoi(limitStr)
		if limit < 1 || limit > 500 {
			limit = 100
		}

		events, err := r.SyncService.GetEventsAfter(c.Request.Context(), lastID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "sync failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"events": events})
	}
}

func (r *Router) handleBanUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			UserID string `json:"user_id" binding:"required"`
			Reason string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := r.ModerationTools.BanUser(c.Request.Context(), body.UserID, body.Reason); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ban failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "banned"})
	}
}

func (r *Router) handleMuteUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			UserID string `json:"user_id" binding:"required"`
			Until  string `json:"until" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := r.ModerationTools.MuteUser(c.Request.Context(), body.UserID, body.Until); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "mute failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "muted"})
	}
}

func (r *Router) handleUploadAvatar() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var body struct {
			MediaID   string `json:"media_id" binding:"required"`
			MediaType string `json:"media_type"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		media := avatar.MediaFileInput{ID: body.MediaID, MediaType: body.MediaType}
		if err := r.AvatarService.UploadAvatar(c.Request.Context(), userID.(string), media); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "avatar upload failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func (r *Router) handleGetProfile() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")
		profile, err := r.AvatarService.GetProfile(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			return
		}
		c.JSON(http.StatusOK, profile)
	}
}

func (r *Router) handleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func (r *Router) handleMetrics() gin.HandlerFunc {
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
