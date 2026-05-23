package adapters

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	message "krampus/internal/message/domain"
	"krampus/internal/message/service"
	"krampus/pkg/apperror"
	"krampus/pkg/ctxmeta"
	"krampus/pkg/types"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 512 * 1024
)

type Client struct {
	ctx       context.Context
	cancel    context.CancelFunc
	meta      ctxmeta.Metadata
	conn      *websocket.Conn
	UserID    types.UserID
	RoomID    types.RoomID
	Send      chan *message.BaseMessage
	msgSvc    *service.MessageService
	mgr       *ConnectionManager
	closeOnce sync.Once
	limiter   *rate.Limiter
}

func NewClient(
	ctx context.Context,
	cancel context.CancelFunc,
	conn *websocket.Conn,
	userID types.UserID,
	roomID types.RoomID,
	meta ctxmeta.Metadata,
	svc *service.MessageService,
	mgr *ConnectionManager,
) *Client {

	conn.EnableWriteCompression(true)
	conn.SetCompressionLevel(flate.BestSpeed)

	return &Client{
		ctx:     ctx,
		cancel:  cancel,
		meta:    meta,
		conn:    conn,
		UserID:  userID,
		RoomID:  roomID,
		Send:    make(chan *message.BaseMessage, 256),
		msgSvc:  svc,
		mgr:     mgr,
		limiter: rate.NewLimiter(20, 40),
	}
}

func (c *Client) ConnID() string {
	return c.conn.RemoteAddr().String() +
		"-" +
		c.UserID.String()
}

func (c *Client) Start() {
	c.safeGo(c.readPump)
	c.safeGo(c.writePump)
}

func (c *Client) safeGo(fn func()) {

	go func() {

		ctx := ctxmeta.WithMetadata(
			c.ctx,
			c.meta,
		)

		defer func() {

			if r := recover(); r != nil {

				meta := ctxmeta.Extract(ctx)

				log.Printf(
					"goroutine panic recovered trace_id=%s user_id=%s panic=%v",
					meta.TraceID,
					meta.UserID,
					r,
				)
			}
		}()

		fn()
	}()
}

func (c *Client) Close(
	code int,
	reason string,
) {

	c.closeOnce.Do(func() {

		c.cancel()

		c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(
				code,
				reason,
			),
			time.Now().Add(time.Second),
		)

		c.conn.Close()

		c.mgr.Unregister(c)
	})
}

func (c *Client) readPump() {

	ctx := ctxmeta.WithMetadata(
		c.ctx,
		c.meta,
	)

	defer c.Close(
		websocket.CloseNormalClosure,
		"connection closed",
	)

	c.conn.SetReadLimit(
		maxMessageSize,
	)

	c.conn.SetReadDeadline(
		time.Now().Add(pongWait),
	)

	c.conn.SetPongHandler(
		func(string) error {

			c.conn.SetReadDeadline(
				time.Now().Add(pongWait),
			)

			return nil
		},
	)

	for {

		select {

		case <-ctx.Done():
			return

		default:
		}

		if !c.limiter.Allow() {

			meta := ctxmeta.Extract(ctx)

			log.Printf(
				"rate limit exceeded trace_id=%s user_id=%s",
				meta.TraceID,
				meta.UserID,
			)

			c.Close(
				websocket.ClosePolicyViolation,
				"rate limit exceeded",
			)

			return
		}

		_, payload, err := c.conn.ReadMessage()

		if err != nil {
			return
		}

		decoder := json.NewDecoder(
			bytes.NewReader(payload),
		)

		decoder.DisallowUnknownFields()

		var msg message.BaseMessage

		if err := decoder.Decode(&msg); err != nil {
			continue
		}

		if msg.Type == message.TypeAckDelivered ||
			msg.Type == message.TypeAckRead {

			var ack message.AckPayload

			if err := json.Unmarshal(msg.Payload, &ack); err != nil {
				continue
			}

			if msg.Type == message.TypeAckDelivered {
				ack.Status = message.DeliveryDelivered
			}

			if msg.Type == message.TypeAckRead {
				ack.Status = message.DeliveryRead
			}

			err := c.mgr.HandleAck(
				ctx,
				c.UserID,
				ack,
			)

			if err != nil {

				log.Printf(
					"ack handle failed trace_id=%s user_id=%s err=%v",
					c.meta.TraceID,
					c.UserID,
					err,
				)
			}

			continue
		}

		msg.UserID = c.UserID
		msg.RoomID = c.RoomID

		msg.Metadata.TraceID = c.meta.TraceID
		msg.Metadata.RequestID = c.meta.RequestID
		msg.Metadata.CorrelationID = c.meta.CorrelationID

		msg.SetTimestamp()

		if err := msg.Validate(); err != nil {
			continue
		}

		if msg.Type.IsRealtimeOnly() {

			err := c.mgr.broadcastRealtime(
				ctx,
				&msg,
			)

			if err != nil {

				meta := ctxmeta.Extract(ctx)

				log.Printf(
					"realtime broadcast failed trace_id=%s user_id=%s err=%v",
					meta.TraceID,
					meta.UserID,
					err,
				)
			}

			continue
		}

		if err := c.msgSvc.Process(
			ctx,
			&msg,
		); err != nil {

			meta := ctxmeta.Extract(ctx)

			log.Printf(
				"message process error trace_id=%s user_id=%s err=%v",
				meta.TraceID,
				meta.UserID,
				err,
			)
		}
	}
}

func (c *Client) writePump() {
	ctx := ctxmeta.WithMetadata(
		c.ctx,
		c.meta,
	)

	ticker := time.NewTicker(
		pingPeriod,
	)

	defer func() {

		ticker.Stop()

		c.Close(
			websocket.CloseNormalClosure,
			"writer stopped",
		)
	}()

	for {

		select {

		case <-ctx.Done():
			return

		case <-ticker.C:

			c.conn.SetWriteDeadline(
				time.Now().Add(writeWait),
			)

			if err := c.conn.WriteMessage(
				websocket.PingMessage,
				nil,
			); err != nil {

				return
			}

		case msg, ok := <-c.Send:

			if !ok {
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			writer, err := c.conn.NextWriter(websocket.TextMessage)

			if err != nil {
				return
			}

			data, _ := json.Marshal(msg)
			writer.Write(data)

			n := len(c.Send)

			for i := 0; i < n; i++ {
				writer.Write([]byte("\n"))
				next := <-c.Send
				data, _ := json.Marshal(next)
				writer.Write(data)
			}

			if err := writer.Close(); err != nil {
				return
			}
		}
	}
}

func (c *Client) SendError(
	appErr *apperror.AppError,
) {

	msg := &message.BaseMessage{
		Type: message.TypeSystem,
		Payload: json.RawMessage(
			fmt.Sprintf(
				`{"code":"%s","message":"%s"}`,
				appErr.Code,
				appErr.Message,
			),
		),
	}

	event, err := BuildEvent(
		time.Now().UnixNano(),
		msg,
	)

	if err != nil {
		return
	}

	if err := c.SafeSend(event); err != nil {

		c.Close(
			websocket.ClosePolicyViolation,
			"slow consumer",
		)
	}
}

func (c *Client) SafeSend(
	event *message.Event,
) error {

	baseMsg := &message.BaseMessage{}

	err := json.Unmarshal(
		event.Payload,
		baseMsg,
	)

	if err != nil {
		return err
	}

	select {

	case <-c.ctx.Done():
		return context.Canceled

	case c.Send <- baseMsg:
		return nil

	default:
		return ErrSlowConsumer
	}
}

func (c *Client) GetUserID() types.UserID {
	return c.UserID
}

func (c *Client) GetRoomID() types.RoomID {
	return c.RoomID
}

func (c *Client) Transport() TransportKind {
	return TransportWebSocket
}
