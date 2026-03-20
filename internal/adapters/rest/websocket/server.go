package websocket

import (
	"net/http"

	"krampus/internal/domain"
	"krampus/pkg/config"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WebSocketServer struct {
	services      *service.Services
	config        *config.Config
	connectionMgr *ConnectionManager
}

func NewWebSocketServer(s *service.Services, cfg *config.Config) *WebSocketServer {
	return &WebSocketServer{
		services:      s,
		config:        cfg,
		connectionMgr: NewConnectionManager(),
	}
}

func (w *WebSocketServer) Start() {
	http.HandleFunc("/ws", w.handleWebSocket)
	println("WS server on /ws")
	http.ListenAndServe(w.config.HTTPPort, nil)
}

func (w *WebSocketServer) handleWebSocket(wr http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(wr, r, nil)
	if err != nil {
		println("Upgrade err:", err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("user_id")
	roomID := r.URL.Query().Get("room_id")
	token := r.URL.Query().Get("token")

	if userID == "" || roomID == "" || token == "" {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "missinng params"))
		return
	}

	if token != "valid" {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid token"))
		return
	}

	client := NewClient(conn, userID, roomID)
	w.connectionMgr.Register(client)
	defer w.connectionMgr.Unregister(client)

	println("WS connected:", userID, roomID)

	for {
		var msg domain.BaseMessage
		err := conn.ReadJSON(&msg)
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			println("WS read err:", err)
			break
		}

		msg.UserID = userID
		msg.RoomID = roomID

		w.services.MessageService.Process(r.Context(), &msg)
		w.connectionMgr.BroadcastToRoom(roomID, &msg)
	}
}
