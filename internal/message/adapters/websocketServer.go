package adapters

import (
	"log"
	"net/http"

	"krampus/internal/message/domain"
	"krampus/internal/message/service"
	"krampus/pkg/config"
	"krampus/pkg/messaging/kafka"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WebSocketServer struct {
	service       *service.MessageService
	config        *config.Config
	connectionMgr *ConnectionManager
}

func NewWebSocketServer(s *service.MessageService, cfg *config.Config, kafkaConsumer *kafka.Consumer) *WebSocketServer {
	return &WebSocketServer{
		service:       s,
		config:        cfg,
		connectionMgr: NewConnectionManager(kafkaConsumer),
	}
}

// func (w *WebSocketServer) Start() {
// 	log.Printf("WS server starting on port %s", w.config.HTTPPort)
// 	mux := http.NewServeMux()
// 	mux.HandleFunc("/ws", w.HandleWebSocket)

// 	if err := http.ListenAndServe(w.config.HTTPPort, mux); err != nil {
// 		log.Fatalf("WS server failed: %v", err)
// 	}
// }

func (w *WebSocketServer) HandleWebSocket(wr http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(wr, r, nil)
	if err != nil {
		println("Upgrade err:", err)
		return
	}

	userID := r.URL.Query().Get("user_id")
	roomID := r.URL.Query().Get("room_id")
	token := r.URL.Query().Get("token")

	if userID == "" || roomID == "" || token == "" {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "missing params"))
		conn.Close()
		return
	}

	if token != "valid" {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid token"))
		conn.Close()
		return
	}

	client := NewClient(conn, userID, roomID, w.service)

	// Регистрируем клиента в менеджере
	if err := w.connectionMgr.Register(client); err != nil {
		log.Printf("Failed to register client: %v", err)
		conn.Close()
		return
	}

	defer w.connectionMgr.Unregister(client)

	log.Printf("WS connected: %s @ %s", userID, roomID)

	for {
		var msg domain.BaseMessage
		err := conn.ReadJSON(&msg)

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS read error: %v", err)
			}
			break
		}

		msg.UserID = userID
		msg.RoomID = roomID

		if err := w.service.Process(r.Context(), &msg); err != nil {
			log.Printf("Message process error: %v", err)
			continue
		}

		w.connectionMgr.BroadcastToRoom(roomID, &msg)
	}
}
