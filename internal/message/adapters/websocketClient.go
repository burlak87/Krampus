package adapters

import (
	"encoding/json"
	"fmt"
	message "krampus/internal/message/domain"
	"krampus/internal/message/service"
	"krampus/pkg/apperror"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn   *websocket.Conn
	UserID string
	RoomID string
	Send   chan *message.BaseMessage
	mu     sync.Mutex
	msgSvc *service.MessageService
}

func NewClient(conn *websocket.Conn, userID, roomID string, svc *service.MessageService) *Client {
	return &Client{
		conn:   conn,
		UserID: userID,
		RoomID: roomID,
		Send:   make(chan *message.BaseMessage, 256),
		msgSvc: svc,
	}
}

func (c *Client) ConnID() string {
	return c.conn.RemoteAddr().String() + "-" + c.UserID
}

func (c *Client) Start() {
	go c.writePump()
	go c.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg message.BaseMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS unexpected close: %v", err)
			}
			break
		}
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		msg.UserID = c.UserID
		msg.RoomID = c.RoomID
	}
}

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for message := range c.Send {
		if message.Type == "ping" {
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			continue
		}

		c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.conn.WriteJSON(message); err != nil {
			log.Printf("WS write error: %v", err)
			return
		}
	}
}

func (c *Client) SendError(appErr *apperror.AppError) {
	errorMsg := &message.BaseMessage{
		Type:    "error",
		Payload: json.RawMessage(fmt.Sprintf(`{"code": "%s", "message": "%s}`, appErr.Code, appErr.Message)),
	}
	select {
	case c.Send <- errorMsg:
	default:
	}
}
