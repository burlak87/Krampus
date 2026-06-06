package adapters

import (
	"context"
	"time"

	"krampus/internal/call/service"
	"krampus/pkg/logging"
	"krampus/pkg/types"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pingPeriod = 50 * time.Second
	sendBuffer = 256
)

// callClient is one connected peer. It implements service.Peer and owns the
// read/write pumps ported from frontend/server/api/main.go (readPump/writePump).
type callClient struct {
	conn   *websocket.Conn
	send   chan []byte
	userID types.UserID
	roomID types.RoomID
	isCall bool
	hub    *service.Hub
	log    *logging.Logger
}

func newCallClient(
	conn *websocket.Conn,
	userID types.UserID,
	roomID types.RoomID,
	isCall bool,
	hub *service.Hub,
) *callClient {
	return &callClient{
		conn:   conn,
		send:   make(chan []byte, sendBuffer),
		userID: userID,
		roomID: roomID,
		isCall: isCall,
		hub:    hub,
		log:    logging.GetLogger(),
	}
}

// service.Peer implementation.
func (c *callClient) UserID() types.UserID { return c.userID }
func (c *callClient) RoomID() types.RoomID { return c.roomID }
func (c *callClient) IsCall() bool         { return c.isCall }

func (c *callClient) Send(data []byte) {
	select {
	case c.send <- data:
	default:
		// Drop on a full buffer — signaling is fire-and-forget; a slow peer must
		// not stall the room (same semantics as the PoC broadcast).
	}
}

// readPump relays every inbound frame to the rest of the room, then tears the
// peer down on disconnect.
func (c *callClient) readPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(ctx, c)
		_ = c.conn.Close()
		close(c.send)
	}()

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.log.Errorf("call: read error room=%s user=%s: %v",
					c.roomID, c.userID, err)
			}
			break
		}
		c.hub.Broadcast(ctx, c.roomID, msg, c)
	}
}

// writePump drains the send channel and keeps the connection alive with pings,
// refreshing presence TTL on each tick.
func (c *callClient) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			if err := c.hub.Refresh(ctx, c); err != nil {
				c.log.Errorf("call: presence refresh failed room=%s user=%s: %v",
					c.roomID, c.userID, err)
			}
		}
	}
}
