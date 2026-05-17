package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	message "krampus/internal/message/domain"
	"krampus/pkg/types"
)

type SSEClient struct {
	ctx           context.Context
	cancel        context.CancelFunc
	UserID        types.UserID
	RoomID        types.RoomID
	subscriptions map[SSEChannel]struct{}
	writer        http.ResponseWriter
	flusher       http.Flusher
	EventSend     chan *message.Event
	closeOnce     sync.Once
}

func NewSSEClient(
	ctx context.Context,
	cancel context.CancelFunc,
	writer http.ResponseWriter,
	userID types.UserID,
	roomID types.RoomID,
) *SSEClient {

	flusher, ok := writer.(http.Flusher)

	if !ok {
		return nil
	}

	return &SSEClient{
		ctx:       ctx,
		cancel:    cancel,
		writer:    writer,
		flusher:   flusher,
		UserID:    userID,
		RoomID:    roomID,
		EventSend: make(chan *message.Event, 256),
		subscriptions: map[SSEChannel]struct{}{
			ChannelMessage:      {},
			ChannelNotification: {},
			ChannelRoomUpdate:   {},
			ChannelPresence:     {},
			ChannelDelivery:     {},
		},
	}
}

func (c *SSEClient) ConnID() string {
	return fmt.Sprintf(
		"sse-%s-%d",
		c.UserID.String(),
		time.Now().UnixNano(),
	)
}

func (c *SSEClient) Start() {
	go c.writePump()
}

func (c *SSEClient) Close() {
	c.closeOnce.Do(func() {
		c.cancel()
	})
}

func (c *SSEClient) writePump() {

	headers := c.writer.Header()

	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")

	c.flusher.Flush()

	ticker := time.NewTicker(
		25 * time.Second,
	)

	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {

		select {

		case <-c.ctx.Done():
			return

		case <-ticker.C:

			_, _ = fmt.Fprintf(
				c.writer,
				": ping\n\n",
			)

			c.flusher.Flush()

		case event, ok := <-c.EventSend:

			if !ok {
				return
			}

			payload, err := json.Marshal(event)

			if err != nil {
				continue
			}

			_, err = fmt.Fprintf(
				c.writer,
				"id: %d\n"+
					"event: %s\n"+
					"data: %s\n\n",
				event.Sequence,
				event.Type,
				payload,
			)

			if err != nil {
				return
			}

			c.flusher.Flush()
		}
	}
}

func (c *SSEClient) SafeSend(
	event *message.Event,
) error {

	select {

	case <-c.ctx.Done():
		return context.Canceled

	case c.EventSend <- event:
		return nil

	default:
		return ErrSlowConsumer
	}
}

func (c *SSEClient) GetUserID() types.UserID {
	return c.UserID
}

func (c *SSEClient) GetRoomID() types.RoomID {
	return c.RoomID
}

func (c *SSEClient) Transport() TransportKind {
	return TransportSSE
}

func (c *SSEClient) IsSubscribed(
	channel SSEChannel,
) bool {

	_, ok := c.subscriptions[channel]

	return ok
}

func (c *SSEClient) SafeSendEvent(
	event *message.Event,
) error {

	select {

	case <-c.ctx.Done():
		return context.Canceled

	case c.EventSend <- event:
		return nil

	default:
		return ErrSlowConsumer
	}
}
