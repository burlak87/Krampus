package adapters

import (
	"context"
	"log"
	"net/http"
	"time"

	identityService "krampus/internal/identity/service"
	message "krampus/internal/message/domain"
	"krampus/internal/message/service"
	database "krampus/internal/sqlc"
	"krampus/pkg/config"
	"krampus/pkg/ctxmeta"
	"krampus/pkg/messaging/kafka"
	"krampus/pkg/types"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:    4096,
	WriteBufferSize:   4096,
	EnableCompression: true,

	CheckOrigin: func(r *http.Request) bool {

		origin := r.Header.Get("Origin")

		allowedOrigins := map[string]bool{
			"http://localhost:3000": true,
			"http://localhost:8000": true,
			"https://yourapp.com":   true,
		}

		return allowedOrigins[origin]
	},
}

type ReplayRepository interface {
	GetMessagesAfterSequence(ctx context.Context, roomID types.RoomID, sequence int64, limit int) ([]*message.BaseMessage, error)
}

type PSQLReplayRepository struct {
	queries *database.Queries
}

type WebSocketServer struct {
	service       *service.MessageService
	config        *config.Config
	connectionMgr *ConnectionManager
	authService   *identityService.WSAuthService
	replayRepo    ReplayRepository
}

func NewWebSocketServer(
	s *service.MessageService,
	cfg *config.Config,
	kafkaConsumer *kafka.Consumer,
	authService *identityService.WSAuthService,
	retryRepo service.RetryRepository,
	dlqRepo service.DLQRepository,
	deliveryRepo DeliveryStatusRepository,
	replayRepo ReplayRepository,
) *WebSocketServer {

	return &WebSocketServer{
		service:       s,
		config:        cfg,
		connectionMgr: NewConnectionManager(kafkaConsumer, retryRepo, dlqRepo, deliveryRepo),
		authService:   authService,
		replayRepo:    replayRepo,
	}
}

func (w *WebSocketServer) Manager() *ConnectionManager {
	return w.connectionMgr
}

func (w *WebSocketServer) HandleWebSocket(
	wr http.ResponseWriter,
	r *http.Request,
) {

	roomID := types.RoomID(
		r.URL.Query().Get("room_id"),
	)

	if roomID == "" {

		http.Error(
			wr,
			"room_id required",
			http.StatusBadRequest,
		)

		return
	}

	authCtx, err := w.authService.Authenticate(
		r.Context(),
		r,
	)

	if err != nil {

		http.Error(
			wr,
			"unauthorized",
			http.StatusUnauthorized,
		)

		return
	}

	conn, err := upgrader.Upgrade(
		wr,
		r,
		nil,
	)

	if err != nil {

		log.Printf(
			"ws upgrade error: %v",
			err,
		)

		return
	}

	meta := ctxmeta.Extract(r.Context())

	meta.UserID = authCtx.UserID.String()

	wsCtx := ctxmeta.WithMetadata(
		context.Background(),
		meta,
	)

	ctx, cancel := context.WithCancel(wsCtx)

	client := NewClient(
		ctx,
		cancel,
		conn,
		authCtx.UserID,
		roomID,
		meta,
		w.service,
		w.connectionMgr,
	)
	client.Start()

	if err := w.connectionMgr.Register(client); err != nil {

		conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(
				websocket.CloseInternalServerErr,
				"registration failed",
			),
			time.Now().Add(time.Second),
		)

		conn.Close()

		return
	}

	log.Printf(
		"ws connected trace_id=%s user_id=%s room_id=%s",
		meta.TraceID,
		authCtx.UserID,
		roomID,
	)
}
