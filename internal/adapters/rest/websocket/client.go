package websocket

import (
	"encoding/json"
	"fmt"
	"krampus/internal/domain"
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
	Send   chan *domain.BaseMessage
	mu     sync.Mutex
}

func NewClient(conn *websocket.Conn, userID, roomID string) *Client {
	return &Client{
		conn:   conn,
		UserID: userID,
		RoomID: roomID,
		Send:   make(chan *domain.BaseMessage, 256),
	}
}

func (c *Client) ConnID() {
	return c.conn.RemoteAddr().String() + "-" + c.UserID
}

func (c *Client) Start() {
	go c.writePump()
	go c.readPump()
}

// func (c *Client) readPump() {
// 	defer c.conn.Close()
// 	for {
// 		var msg domain.BaseMessage
// 		err := c.conn.ReadJSON(&msg)
// 		if err != nil {
// 			break
// 		}
// 		msg.UserID = c.UserID
// 		msg.RoomID = c.RoomID
// 		// Process via services (stub log)
// 		println("Msg from", c.UserID, ":", string(msg.Payload))
// 	}
// }

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
		var msg domain.BaseMessage
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

// func (c *Client) writePump() {
// 	defer c.conn.Close()
// 	for msg := range c.Send {
// 		if err := c.conn.WriteJSON(msg); err != nil {
// 			break
// 		}
// 	}
// }

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.conn.Close()
				return
			}

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
}

func (c *Client) SendError(appErr *apperror.AppError) {
	errorMsg := &domain.BaseMessage{
		Type:    "error",
		Payload: json.RawMessage(fmt.Sprintf(`{"code": "%s", "message": "%s}`, appErr.Code, appErr.Message)),
	}
	select {
	case c.Send <- errorMsg:
	default:
	}
}
