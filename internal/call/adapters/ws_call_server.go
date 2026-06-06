package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"krampus/internal/call/domain"
	"krampus/internal/call/service"
	identityDomain "krampus/internal/identity/domain"
	"krampus/pkg/ctxmeta"
	"krampus/pkg/logging"
	"krampus/pkg/types"

	"github.com/gorilla/websocket"
)

// Authenticator validates a WebSocket upgrade request (JWT + session + room
// membership). *identity/service.WSAuthService satisfies it; a fake is used in
// tests. Defined as an interface here so the handler can be exercised without a
// live Redis/Postgres-backed auth service.
type Authenticator interface {
	Authenticate(ctx context.Context, r *http.Request) (*identityDomain.AuthContext, error)
}

// allowedOrigins mirrors the whitelist used by the message WS upgrader so the
// Nuxt dev client (frontend/) can connect.
var allowedOrigins = map[string]bool{
	"http://localhost:3000": true,
	"http://localhost:8000": true,
	"https://yourapp.com":   true,
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return allowedOrigins[r.Header.Get("Origin")]
	},
}

// CallWSServer is the WebRTC signaling endpoint migrated from
// frontend/server/api/main.go, now gated by WSAuthService and backed by the Hub.
type CallWSServer struct {
	hub         *service.Hub
	authService Authenticator
	log         *logging.Logger
}

func NewCallWSServer(hub *service.Hub, authService Authenticator) *CallWSServer {
	return &CallWSServer{hub: hub, authService: authService, log: logging.GetLogger()}
}

// HandleCallWebSocket serves GET /call/ws?room_id=<id>&type=call&token=<jwt>.
func (s *CallWSServer) HandleCallWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := types.RoomID(r.URL.Query().Get("room_id"))
	if roomID == "" {
		http.Error(w, "room_id required", http.StatusBadRequest)
		return
	}

	isCall := r.URL.Query().Get("type") == domain.TypeWSCall

	authCtx, err := s.authService.Authenticate(r.Context(), r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Errorf("call: ws upgrade error: %v", err)
		return
	}

	meta := ctxmeta.Extract(r.Context())
	meta.UserID = authCtx.UserID.String()
	ctx, cancel := context.WithCancel(ctxmeta.WithMetadata(context.Background(), meta))

	// For call connections, answer the bootstrap count frame BEFORE registering
	// this peer, so the client learns how many peers were already present
	// (excluding itself) — identical handshake to the PoC.
	if isCall {
		if !s.answerCountBootstrap(ctx, conn, roomID, authCtx.UserID) {
			cancel()
			_ = conn.Close()
			return
		}
	}

	client := newCallClient(conn, authCtx.UserID, roomID, isCall, s.hub)
	if err := s.hub.Register(ctx, client); err != nil {
		if errors.Is(err, service.ErrRoomFull) {
			_ = conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "room full"))
		} else {
			s.log.Errorf("call: register failed room=%s user=%s: %v", roomID, authCtx.UserID, err)
		}
		cancel()
		_ = conn.Close()
		return
	}

	s.log.Infof("call: connected user=%s room=%s is_call=%v", authCtx.UserID, roomID, isCall)

	go func() {
		client.readPump(ctx)
		cancel()
	}()
	go client.writePump(ctx)
}

// answerCountBootstrap reads the first frame; if it is a checkCountUserCall it
// replies with the current call-peer count. Returns false if the connection
// should be aborted (read failure).
func (s *CallWSServer) answerCountBootstrap(
	ctx context.Context,
	conn *websocket.Conn,
	roomID types.RoomID,
	userID types.UserID,
) bool {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return false
	}

	var env domain.Envelope
	if json.Unmarshal(msg, &env) != nil || env.Type != domain.TypeCheckCount {
		// Not a bootstrap frame — treat as ignorable; clients always send
		// checkCountUserCall first.
		return true
	}

	count, err := s.hub.CallCount(ctx, roomID, userID)
	if err != nil {
		s.log.Errorf("call: count failed room=%s: %v", roomID, err)
		count = 0
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteMessage(websocket.TextMessage, service.CheckCountResponse(count)); err != nil {
		return false
	}
	return true
}
