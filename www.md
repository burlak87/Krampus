internal/message/adapters/event_builder.go:
package adapters

import (
	"encoding/json"

	message "krampus/internal/message/domain"
)

func BuildEvent(
	seq int64,
	msg *message.BaseMessage,
) (*message.Event, error) {

	payload, err := json.Marshal(msg)

	if err != nil {
		return nil, err
	}

	eventType := "message"

	switch msg.Type {

	case message.TypePresenceRealtime:
		eventType = "presence"

	case message.TypeSystem:
		eventType = "room_update"
	}

	return &message.Event{
		ID:          string(msg.ID),
		Sequence:    seq,
		Type:        eventType,
		AggregateID: string(msg.RoomID),
		UserID:      msg.UserID,
		RoomID:      msg.RoomID,
		Timestamp:   msg.Timestamp,
		Payload:     payload,
	}, nil
}

internal/message/adapters/pubsub.go:
package adapters

import (
	"context"

	message "krampus/internal/message/domain"
)

type PubSub interface {
	Publish(
		ctx context.Context,
		channel string,
		msg *message.BaseMessage,
	) error

	Subscribe(
		ctx context.Context,
		channel string,
		handler func(*message.BaseMessage),
	) error
}

internal/message/adapters/sequencer.go:
package adapters

import "sync/atomic"

type EventSequencer struct {
	sequence atomic.Int64
}

func NewEventSequencer() *EventSequencer {
	return &EventSequencer{}
}

func (s *EventSequencer) Next() int64 {
	return s.sequence.Add(1)
}

internal/message/adapters/sse_channels.go:
package adapters

type SSEChannel string

const (
	ChannelMessage      SSEChannel = "message"
	ChannelNotification SSEChannel = "notification"
	ChannelRoomUpdate   SSEChannel = "room_update"
	ChannelPresence     SSEChannel = "presence"
	ChannelDelivery     SSEChannel = "delivery"
)

func EventToChannel(
	eventType string,
) SSEChannel {

	switch eventType {

	case "message":
		return ChannelMessage

	case "notification":
		return ChannelNotification

	case "room_update":
		return ChannelRoomUpdate

	case "presence":
		return ChannelPresence

	case "delivery":
		return ChannelDelivery

	default:
		return ChannelMessage
	}
}

internal/message/adapters/SSEClient.go:
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
	Send          chan *message.Event
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
		Send:      make(chan *message.Event, 256),
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

	case c.Send <- event:
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

internal/message/adapters/SSEServer.go:
package adapters

import (
	"context"
	"log"
	"net/http"
	"strconv"

	identityService "krampus/internal/identity/service"
	"krampus/pkg/ctxmeta"
	"krampus/pkg/types"
)

type SSEServer struct {
	authService *identityService.WSAuthService
	manager     *ConnectionManager
	replayRepo  ReplayRepository
}

func NewSSEServer(
	authService *identityService.WSAuthService,
	manager *ConnectionManager,
	replayRepo ReplayRepository,
) *SSEServer {

	return &SSEServer{
		authService: authService,
		manager:     manager,
		replayRepo:  replayRepo,
	}
}

func (s *SSEServer) HandleSSE(
	w http.ResponseWriter,
	r *http.Request,
) {

	roomID := types.RoomID(
		r.URL.Query().Get("room_id"),
	)

	if roomID == "" {

		http.Error(
			w,
			"room_id required",
			http.StatusBadRequest,
		)

		return
	}

	authCtx, err := s.authService.Authenticate(
		r.Context(),
		r,
	)

	if err != nil {

		http.Error(
			w,
			"unauthorized",
			http.StatusUnauthorized,
		)

		return
	}

	meta := ctxmeta.Extract(
		r.Context(),
	)

	meta.UserID = authCtx.UserID.String()

	ctx := ctxmeta.WithMetadata(
		context.Background(),
		meta,
	)

	clientCtx, cancel := context.WithCancel(
		ctx,
	)

	client := NewSSEClient(
		clientCtx,
		cancel,
		w,
		authCtx.UserID,
		roomID,
	)

	if client == nil {

		http.Error(
			w,
			"sse unsupported",
			http.StatusInternalServerError,
		)

		return
	}

	if err := s.manager.Register(client); err != nil {

		http.Error(
			w,
			"registration failed",
			http.StatusInternalServerError,
		)

		return
	}

	defer s.manager.Unregister(client)

	client.Start()

	lastEventID := r.Header.Get(
		"Last-Event-ID",
	)

	if lastEventID == "" {

		lastEventID = r.URL.Query().
			Get("last_event_id")
	}

	if lastEventID != "" {

		lastSequence, err := strconv.ParseInt(
			lastEventID,
			10,
			64,
		)

		if err == nil {

			messages, err := s.replayRepo.
				GetMessagesAfterSequence(
					clientCtx,
					roomID,
					lastSequence,
					100,
				)

			if err == nil {

				for _, replayMsg := range messages {

					event, err := BuildEvent(
						replayMsg.Timestamp,
						replayMsg,
					)

					if err != nil {
						continue
					}

					err = client.SafeSend(
						event,
					)

					if err != nil {
						break
					}
				}
			}
		}
	}

	log.Printf(
		"sse connected trace_id=%s user_id=%s room_id=%s",
		meta.TraceID,
		authCtx.UserID,
		roomID,
	)

	<-clientCtx.Done()
}

internal/message/adapters/transport.go:
package adapters

import (
	message "krampus/internal/message/domain"
	"krampus/pkg/types"
)

type TransportClient interface {
	ConnID() string

	SafeSend(
		event *message.Event,
	) error

	GetUserID() types.UserID

	GetRoomID() types.RoomID

	Transport() TransportKind
}

internal/message/adapters/transport_capabilities.go:
package adapters

type TransportKind string

const (
	TransportWebSocket TransportKind = "websocket"
	TransportSSE       TransportKind = "sse"
)

internal/message/adapters/WebSocketClient.go:
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

			c.conn.SetWriteDeadline(
				time.Now().Add(writeWait),
			)

			writer, err := c.conn.NextWriter(
				websocket.TextMessage,
			)

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

internal/message/adapters/WebSocketManager.go:
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	message "krampus/internal/message/domain"
	"krampus/internal/message/service"
	"krampus/pkg/ctxmeta"
	"krampus/pkg/messaging/kafka"
	"krampus/pkg/types"
)

const (
	deliveryWorkers  = 32
	maxRetryAttempts = 5
)

var ErrSlowConsumer = errors.New(
	"slow consumer",
)

type DeliveryStatusRepository interface {
	UpdateStatus(
		ctx context.Context,
		messageID types.MessageID,
		userID types.UserID,
		status message.DeliveryStatus,
	) error

	MarkDelivered(
		ctx context.Context,
		messageID types.MessageID,
		userID types.UserID,
		at time.Time,
	) error

	MarkRead(
		ctx context.Context,
		messageID types.MessageID,
		userID types.UserID,
		at time.Time,
	) error
}

type ConnectionManager struct {
	users         sync.Map
	rooms         sync.Map
	deliveryQueue chan *message.DeliveryJob
	kafkaConsumer *kafka.Consumer
	retryRepo     service.RetryRepository
	dlqRepo       service.DLQRepository
	deliveryRepo  DeliveryStatusRepository
	sequencer     *EventSequencer
}

type UserConnections struct {
	mu    sync.RWMutex
	conns map[string]TransportClient
}

type RoomSubscribers struct {
	mu    sync.RWMutex
	users map[types.UserID]map[string]TransportClient
}

func NewConnectionManager(
	kafkaConsumer *kafka.Consumer,
	retryRepo service.RetryRepository,
	dlqRepo service.DLQRepository,
	deliveryRepo DeliveryStatusRepository,
) *ConnectionManager {

	m := &ConnectionManager{
		deliveryQueue: make(
			chan *message.DeliveryJob,
			8192,
		),
		kafkaConsumer: kafkaConsumer,
		retryRepo:     retryRepo,
		dlqRepo:       dlqRepo,
		deliveryRepo:  deliveryRepo,
		sequencer:     NewEventSequencer(),
	}

	for i := 0; i < deliveryWorkers; i++ {
		go m.deliveryWorker(i)
	}

	retryWorker := service.NewRetryWorker(
		retryRepo,
		dlqRepo,
		m,
	)

	go retryWorker.Start(
		context.Background(),
	)

	m.kafkaConsumer.AddHandler(
		func(msg *message.BaseMessage) {

			ctx := ctxmeta.WithMetadata(
				context.Background(),
				ctxmeta.Metadata{
					TraceID:       msg.Metadata.TraceID,
					RequestID:     msg.Metadata.RequestID,
					CorrelationID: msg.Metadata.CorrelationID,
					UserID:        msg.UserID.String(),
				},
			)

			m.RouteMessage(
				ctx,
				msg,
			)
		},
	)

	go m.kafkaConsumer.Consume(
		context.Background(),
	)

	return m
}

func (m *ConnectionManager) Register(
	client TransportClient,
) error {

	userIDI, _ := m.users.LoadOrStore(
		client.GetUserID(),
		&UserConnections{
			conns: make(
				map[string]TransportClient,
			),
		},
	)

	uc := userIDI.(*UserConnections)

	uc.mu.Lock()
	uc.conns[client.ConnID()] = client
	uc.mu.Unlock()

	m.subscribeToRoom(
		client.GetRoomID(),
		client.GetUserID(),
		client,
	)

	return nil
}

func (m *ConnectionManager) Unregister(
	client TransportClient,
) {

	if ucI, ok := m.users.Load(
		client.GetUserID(),
	); ok {

		uc := ucI.(*UserConnections)

		uc.mu.Lock()

		delete(
			uc.conns,
			client.ConnID(),
		)

		empty := len(uc.conns) == 0

		uc.mu.Unlock()

		if empty {
			m.users.Delete(
				client.GetUserID(),
			)
		}
	}

	m.unsubscribeFromRoom(
		client.GetRoomID(),
		client.GetUserID(),
		client,
	)
}

func (m *ConnectionManager) RouteMessage(
	ctx context.Context,
	msg *message.BaseMessage,
) error {

	if msg.Type.IsRealtimeOnly() {

		return m.broadcastRealtime(
			ctx,
			msg,
		)
	}

	return m.broadcastDurable(
		ctx,
		msg,
	)
}

func (m *ConnectionManager) broadcastRealtime(
	ctx context.Context,
	msg *message.BaseMessage,
) error {

	subsI, ok := m.rooms.Load(
		msg.RoomID,
	)

	if !ok {
		return nil
	}

	subs := subsI.(*RoomSubscribers)

	subs.mu.RLock()

	clients := make(
		[]TransportClient,
		0,
	)

	for _, userClients := range subs.users {

		for _, client := range userClients {

			if client.Transport() != TransportWebSocket {
				continue
			}

			clients = append(
				clients,
				client,
			)
		}
	}

	subs.mu.RUnlock()

	for _, client := range clients {

		job := &message.DeliveryJob{
			Message:  msg,
			ClientID: client.ConnID(),
			UserID:   client.GetUserID(),
			RoomID:   client.GetRoomID(),
		}

		select {

		case m.deliveryQueue <- job:

		default:

			log.Printf(
				"realtime delivery overflow room_id=%s",
				msg.RoomID,
			)
		}
	}

	return nil
}

func (m *ConnectionManager) broadcastDurable(
	ctx context.Context,
	msg *message.BaseMessage,
) error {

	event, err := BuildEvent(
		m.sequencer.Next(),
		msg,
	)

	if err != nil {
		return err
	}

	subsI, ok := m.rooms.Load(
		msg.RoomID,
	)

	if !ok {
		return nil
	}

	subs := subsI.(*RoomSubscribers)

	subs.mu.RLock()

	clients := make(
		[]TransportClient,
		0,
	)

	for _, userClients := range subs.users {

		for _, client := range userClients {

			switch c := client.(type) {

			case *SSEClient:

				channel := EventToChannel(
					event.Type,
				)

				if !c.IsSubscribed(channel) {
					continue
				}

				err := c.SafeSendEvent(
					event,
				)

				if err != nil {

					log.Printf(
						"sse send failed user_id=%s err=%v",
						c.GetUserID(),
						err,
					)
				}

			default:

				clients = append(
					clients,
					client,
				)
			}
		}
	}

	subs.mu.RUnlock()

	for _, client := range clients {

		job := &message.DeliveryJob{
			Message:  msg,
			ClientID: client.ConnID(),
			UserID:   client.GetUserID(),
			RoomID:   client.GetRoomID(),
		}

		select {

		case m.deliveryQueue <- job:

		default:

			log.Printf(
				"durable delivery overflow room_id=%s",
				msg.RoomID,
			)
		}
	}

	return nil
}

func (m *ConnectionManager) SendToUser(
	ctx context.Context,
	userID types.UserID,
	msg *message.BaseMessage,
) error {

	event, err := BuildEvent(
		m.sequencer.Next(),
		msg,
	)

	if err != nil {
		return err
	}

	ucI, ok := m.users.Load(
		userID,
	)

	if !ok {
		return nil
	}

	uc := ucI.(*UserConnections)

	uc.mu.RLock()

	defer uc.mu.RUnlock()

	for _, client := range uc.conns {

		switch c := client.(type) {

		case *SSEClient:

			channel := EventToChannel(
				event.Type,
			)

			if !c.IsSubscribed(channel) {
				continue
			}

			err := c.SafeSendEvent(
				event,
			)

			if err != nil {

				log.Printf(
					"user sse send failed user_id=%s err=%v",
					userID,
					err,
				)
			}

		default:

			job := &message.DeliveryJob{
				Message:  msg,
				ClientID: client.ConnID(),
				UserID:   client.GetUserID(),
				RoomID:   client.GetRoomID(),
			}

			select {

			case m.deliveryQueue <- job:

			default:

				log.Printf(
					"user delivery overflow user_id=%s",
					userID,
				)
			}
		}
	}

	return nil
}

func (m *ConnectionManager) deliveryWorker(
	workerID int,
) {

	for job := range m.deliveryQueue {

		ctx := ctxmeta.WithMetadata(
			context.Background(),
			ctxmeta.Metadata{
				TraceID:       job.Message.Metadata.TraceID,
				RequestID:     job.Message.Metadata.RequestID,
				CorrelationID: job.Message.Metadata.CorrelationID,
				UserID:        job.UserID.String(),
			},
		)

		if err := m.ProcessDelivery(
			ctx,
			job,
		); err != nil {

			job.Attempt++

			if job.Attempt >= maxRetryAttempts {

				m.sendFailedAck(
					ctx,
					job,
				)

				_ = m.dlqRepo.Store(
					ctx,
					job,
					"max retry attempts reached",
				)

				continue
			}

			delay := time.Second *
				time.Duration(
					1<<job.Attempt,
				)

			job.NextRetryAt = time.Now().
				Add(delay)

			err := m.retryRepo.Enqueue(
				ctx,
				job,
			)

			if err != nil {
				return
			}
		}
	}
}

func (m *ConnectionManager) sendSentAck(
	client TransportClient,
	msg *message.BaseMessage,
) {

	if _, ok := client.(*SSEClient); ok {
		return
	}

	payload, _ := json.Marshal(
		message.AckPayload{
			MessageID: msg.ID,
			Status:    message.DeliverySent,
			Timestamp: time.Now().UnixNano(),
		},
	)

	ack := &message.BaseMessage{
		ID: types.MessageID(
			time.Now().Format(
				time.RFC3339Nano,
			),
		),
		Type:      message.TypeAckSent,
		UserID:    client.GetUserID(),
		RoomID:    client.GetRoomID(),
		Timestamp: time.Now().UnixNano(),
		Payload:   payload,
	}

	event, err := BuildEvent(
		time.Now().UnixNano(),
		ack,
	)

	if err == nil {
		_ = client.SafeSend(event)
	}
}

func (m *ConnectionManager) sendFailedAck(ctx context.Context, job *message.DeliveryJob) {
	err := m.deliveryRepo.UpdateStatus(ctx, job.Message.ID, job.UserID, message.DeliveryFailed)
	if err != nil {
		log.Printf("failed to persist failed delivery message_id=%s user_id=%s err=%v", job.Message.ID, job.UserID, err)
	}

	payload, _ := json.Marshal(
		message.AckPayload{
			MessageID: job.Message.ID,
			Status:    message.DeliveryFailed,
			Timestamp: time.Now().UnixNano(),
		},
	)

	log.Printf("delivery failed payload=%s", string(payload))
}

func (m *ConnectionManager) findClient(
	userID types.UserID,
	clientID string,
) (TransportClient, error) {

	ucI, ok := m.users.Load(userID)

	if !ok {
		return nil, errors.New("user not connected")
	}

	uc := ucI.(*UserConnections)

	uc.mu.RLock()

	defer uc.mu.RUnlock()

	client, ok := uc.conns[clientID]

	if !ok {
		return nil, errors.New("client not found")
	}

	return client, nil
}

func (m *ConnectionManager) subscribeToRoom(
	roomID types.RoomID,
	userID types.UserID,
	client TransportClient,
) {

	subsI, _ := m.rooms.LoadOrStore(
		roomID,
		&RoomSubscribers{
			users: make(
				map[types.UserID]map[string]TransportClient,
			),
		},
	)

	subs := subsI.(*RoomSubscribers)

	subs.mu.Lock()

	if _, ok := subs.users[userID]; !ok {

		subs.users[userID] = make(
			map[string]TransportClient,
		)
	}

	subs.users[userID][client.ConnID()] = client

	subs.mu.Unlock()
}

func (m *ConnectionManager) unsubscribeFromRoom(roomID types.RoomID, userID types.UserID, client TransportClient) {
	subsI, ok := m.rooms.Load(roomID)
	if !ok {
		return
	}
	subs := subsI.(*RoomSubscribers)
	subs.mu.Lock()

	if userClients, ok := subs.users[userID]; ok {
		delete(userClients, client.ConnID())
		if len(userClients) == 0 {
			delete(subs.users, userID)
		}
	}

	empty := len(subs.users) == 0
	subs.mu.Unlock()
	if empty {
		m.rooms.Delete(roomID)
	}
}

func (m *ConnectionManager) ProcessDelivery(ctx context.Context, job *message.DeliveryJob) error {
	client, err := m.findClient(job.UserID, job.ClientID)
	if err != nil {
		return err
	}

	event, err := BuildEvent(
		time.Now().UnixNano(),
		job.Message,
	)

	if err != nil {
		return err
	}

	if err := client.SafeSend(event); err != nil {
		return err
	}

	m.sendSentAck(client, job.Message)

	return nil

	// select {
	// 	if err := client.SafeSend(job.Message); err != nil {
	// 		return err
	// 	}
	// 	err := m.deliveryRepo.UpdateStatus(ctx, job.Message.ID, job.UserID, message.DeliverySent)
	// 	if err != nil {
	// 		log.Printf("delivery sent persist failed message_id=%s user_id=%s err=%v", job.Message.ID, job.UserID, err)
	// 	}

	// 	m.sendSentAck(client, job.Message)
	// 	return nil
	// default:
	// 	client.Close(websocket.ClosePolicyViolation, "slow consumer")
	// 	return ErrSlowConsumer
	// }
}

func (m *ConnectionManager) HandleAck(ctx context.Context, userID types.UserID, ack message.AckPayload) error {
	switch ack.Status {
	case message.DeliveryDelivered:
		return m.deliveryRepo.MarkDelivered(ctx, ack.MessageID, userID, time.Now())
	case message.DeliveryRead:
		return m.deliveryRepo.MarkRead(ctx, ack.MessageID, userID, time.Now())
	}

	return nil
}

func (m *ConnectionManager) SendPresenceUpdate(
	ctx context.Context,
	roomID types.RoomID,
	msg *message.BaseMessage,
) error {

	subsI, ok := m.rooms.Load(
		roomID,
	)

	if !ok {
		return nil
	}

	subs := subsI.(*RoomSubscribers)

	subs.mu.RLock()

	clients := make(
		[]TransportClient,
		0,
	)

	for _, userClients := range subs.users {

		for _, client := range userClients {

			if client.Transport() != TransportSSE {
				continue
			}

			sseClient, ok := client.(*SSEClient)

			if !ok {
				continue
			}

			if !sseClient.IsSubscribed(
				ChannelPresence,
			) {
				continue
			}

			clients = append(
				clients,
				client,
			)
		}
	}

	subs.mu.RUnlock()

	for _, client := range clients {

		job := &message.DeliveryJob{
			Message:  msg,
			ClientID: client.ConnID(),
			UserID:   client.GetUserID(),
			RoomID:   client.GetRoomID(),
		}

		select {

		case m.deliveryQueue <- job:

		default:

			log.Printf(
				"presence overflow room_id=%s",
				roomID,
			)
		}
	}

	return nil
}

internal/message/adapters/WebSocketServer.go:
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

internal/message/domain/message.go:
package domain

import (
	"encoding/json"
	"errors"
	"krampus/pkg/types"
	"time"

	"github.com/google/uuid"
)

type MessageType string

const (
	TypeText        MessageType = "text"
	TypeFile        MessageType = "file"
	TypeCommand     MessageType = "command"
	TypeSystem      MessageType = "system"
	TypeVideoCall   MessageType = "video_call"
	TypeTyping      MessageType = "typing"
	TypeReadReceipt MessageType = "read_receipt"

	TypeCursor           MessageType = "cursor"
	TypePresenceRealtime MessageType = "presence_realtime"
	TypeGame             MessageType = "game"
	TypeWebRTCSignal     MessageType = "webrtc_signal"

	TypeAck          MessageType = "ack"
	TypeAckSent      MessageType = "ack_sent"
	TypeAckDelivered MessageType = "ack_delivered"
	TypeAckRead      MessageType = "ack_read"
	TypeAckFailed    MessageType = "ack_failed"
)

type BaseMessage struct {
	ID        types.MessageID `json:"id"`
	Type      MessageType     `json:"type"`
	UserID    types.UserID    `json:"user_id"`
	RoomID    types.RoomID    `json:"room_id"`
	Timestamp int64           `json:"timestamp"`
	Version   int             `json:"version"`
	Payload   json.RawMessage `json:"payload"`
	Metadata  Metadata        `json:"metadata"`
	Signature string          `json:"signature"`
}

type TextPayload struct {
	Text      string              `json:"text"`
	Format    string              `json:"format"`
	Mentions  []string            `json:"mentions"`
	ReplyTo   string              `json:"reply_to"`
	Reactions map[string][]string `json:"reactions"`
}

type CommandPayload struct {
	Cmd     string                 `json:"cmd"`
	Args    []string               `json:"args"`
	Options map[string]interface{} `json:"options"`
}

type SystemPayload struct {
	Actions string      `json:"actions"`
	Data    interface{} `json:"data"`
}

type Metadata struct {
	TraceID       string            `json:"trace_id,omitempty"`
	RequestID     string            `json:"request_id,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Version       int               `json:"version"`
	Compression   string            `json:"compression,omitempty"`
	Encryption    string            `json:"encryption,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

var ErrInvalidTimestamp = errors.New("invalid timestamp")

func (m *BaseMessage) Validate() error {
	if m.ID == "" || m.UserID == "" || m.RoomID == "" {
		return errors.New("missing required fields")
	}
	if m.Timestamp <= 0 {
		return ErrInvalidTimestamp
	}
	if m.Timestamp > time.Now().Add(5*time.Minute).UnixNano() {
		return ErrInvalidTimestamp
	}
	return nil
}

func (m *BaseMessage) SetTimestamp() {
	m.Timestamp = time.Now().UnixNano()
	m.ID = types.MessageID(uuid.New().String())
}

func NewTextMessage(userID types.UserID, roomID types.RoomID, text string) *BaseMessage {
	msg := &BaseMessage{
		Type:    TypeText,
		UserID:  userID,
		RoomID:  roomID,
		Version: 1,
	}
	payload, _ := json.Marshal(TextPayload{
		Text: text,
	})
	msg.Payload = payload
	msg.SetTimestamp()
	return msg
}

func (t MessageType) IsRealtimeOnly() bool {
	switch t {
	case TypeTyping,
		TypeVideoCall,
		TypeCursor,
		TypePresenceRealtime,
		TypeGame,
		TypeWebRTCSignal:
		return true
	default:
		return false
	}
}

func (m *BaseMessage) ToEvent(sequence int64) (*Event, error) {
	payload, err := json.Marshal(m)

	if err != nil {
		return nil, err
	}
	return &Event{
		ID:          string(m.ID),
		Sequence:    sequence,
		Type:        string(m.Type),
		AggregateID: string(m.RoomID),
		UserID:      m.UserID,
		RoomID:      m.RoomID,
		Timestamp:   m.Timestamp,
		Payload:     payload,
	}, nil
}

internal/message/domain/event.go:
package domain

import (
	"encoding/json"
	"krampus/pkg/types"
)

type Event struct {
	ID          string          `json:"id"`
	Sequence    int64           `json:"sequence"`
	Type        string          `json:"type"`
	AggregateID string          `json:"aggregate_id"`
	UserID      types.UserID    `json:"user_id,omitempty"`
	RoomID      types.RoomID    `json:"room_id,omitempty"`
	Timestamp   int64           `json:"timestamp"`
	Payload     json.RawMessage `json:"payload"`
}

internal/message/domain/delivery.go:
package domain

import (
	"time"

	"krampus/pkg/types"
)

type DeliveryStatus string

const (
	DeliverySent      DeliveryStatus = "sent"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryRead      DeliveryStatus = "read"
	DeliveryFailed    DeliveryStatus = "failed"
)

type DeliveryJob struct {
	Message *BaseMessage

	ClientID string

	UserID types.UserID
	RoomID types.RoomID

	Attempt int

	NextRetryAt time.Time
}

type AckPayload struct {
	MessageID types.MessageID `json:"message_id"`
	Status    DeliveryStatus  `json:"status"`
	Timestamp int64           `json:"timestamp"`
}


internal/message/domain/outbox.go:
package domain

import (
	"time"

	"krampus/pkg/types"
)

type OutboxEvent struct {
	ID string

	AggregateType string
	AggregateID   string

	EventType string

	Payload []byte

	CreatedAt time.Time

	PublishedAt *time.Time

	RetryCount int

	LastError string
}

const (
	OutboxEventMessageCreated = "message.created"
)

type MessageCreatedEvent struct {
	MessageID types.MessageID `json:"message_id"`
	RoomID    types.RoomID    `json:"room_id"`
	UserID    types.UserID    `json:"user_id"`
}

internal/message/domain/pipeline.go:
package domain

import "time"

type PipelineJob struct {
	Message     *BaseMessage
	Attempt     int
	CreatedAt   time.Time
	NextRetryAt time.Time
}

internal/message/service/messageService.go:
package service

import (
	"context"
	"encoding/json"
	"log"

	chat "krampus/internal/chat/service"
	message "krampus/internal/message/domain"
	messageStorage "krampus/internal/message/storage"
	"krampus/pkg/apperror"
	"krampus/pkg/types"

	"github.com/google/uuid"
)

type MessageStorage interface {
	SaveMessage(ctx context.Context, msg *message.BaseMessage) error
	SaveMessageBatch(ctx context.Context, msgs []*message.BaseMessage) error
	GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*message.BaseMessage, error)
	GetMessage(ctx context.Context, id string) (*message.BaseMessage, error)
}

type MessageDistributor interface {
	Broadcast(ctx context.Context, msg *message.BaseMessage) error
	BroadcastToRoom(ctx context.Context, msg *message.BaseMessage, roomID string) error
	SendToUserClient(ctx context.Context, userID string, msg *message.BaseMessage) error
}

type IdempotencyRepository interface {
	IsDuplicate(ctx context.Context, key string) (bool, error)
	Save(ctx context.Context, key string, messageID string) error
}

type MessageService struct {
	storage         *messageStorage.MessagePGStorage
	distributor     *messageStorage.MessageDistributor
	roomSvc         *chat.RoomService
	userClientSvc   *chat.UserClientService
	rateLimiter     *RateLimiter
	outboxRepo      OutboxRepository
	idempotencyRepo IdempotencyRepository
}

func NewMessageService(storage *messageStorage.MessagePGStorage, dist *messageStorage.MessageDistributor, roomSvc *chat.RoomService, userClientSvc *chat.UserClientService, outboxRepo OutboxRepository, idempotencyRepo IdempotencyRepository) *MessageService {
	return &MessageService{
		storage:         storage,
		distributor:     dist,
		roomSvc:         roomSvc,
		userClientSvc:   userClientSvc,
		rateLimiter:     NewRateLimiter(),
		outboxRepo:      outboxRepo,
		idempotencyRepo: idempotencyRepo,
	}
}

func (ms *MessageService) Process(ctx context.Context, msg *message.BaseMessage) error {
	if msg.Timestamp == 0 {
		msg.SetTimestamp()
	}
	if err := msg.Validate(); err != nil {
		return apperror.New(apperror.ErrInvalidMessage, err.Error())
	}

	if err := ms.rateLimiter.Check(ctx, msg.UserID.String(), msg.Type); err != nil {
		return err
	}

	user, err := ms.userClientSvc.GetUser(ctx, msg.UserID.String())
	if err != nil {
		return apperror.New(apperror.ErrUserNotFound, "user client not found")
	}

	room, err := ms.roomSvc.GetRoom(ctx, msg.RoomID.String())
	if err != nil {
		return apperror.New(apperror.ErrRoomNotFound, "room not found")
	}
	if !ms.roomSvc.CanSendMessage(ctx, room, user, msg.Type) {
		return apperror.New(apperror.ErrForbidden, "no permission to send")
	}

	idempotencyKey := msg.ID.String()

	duplicate, err := ms.idempotencyRepo.
		IsDuplicate(ctx, idempotencyKey)
	if err != nil {
		return apperror.New(apperror.ErrInternal, "idempotency check failed")
	}

	if duplicate {
		log.Printf("duplicate message skipped message_id=%s", msg.ID)
		return nil
	}

	err = ms.storage.SaveMessage(ctx, msg)

	if err != nil {
		return apperror.New(apperror.ErrStorage, "failed to save message")
	}

	err = ms.idempotencyRepo.Save(ctx, idempotencyKey, msg.ID.String())

	if err != nil {
		return apperror.New(apperror.ErrInternal, "failed to save idempotency key")
	}

	eventPayload, err := json.Marshal(message.MessageCreatedEvent{
		MessageID: msg.ID,
		RoomID:    msg.RoomID,
		UserID:    msg.UserID,
	})

	if err != nil {
		return apperror.New(apperror.ErrInternal, "failed to marshal outbox payload")
	}

	event := &message.OutboxEvent{
		ID:            uuid.NewString(),
		AggregateType: "message",
		AggregateID:   msg.ID.String(),
		EventType:     message.OutboxEventMessageCreated,
		Payload:       eventPayload,
	}

	err = ms.outboxRepo.SaveEvent(ctx, event)

	if err != nil {
		return apperror.New(apperror.ErrInternal, "failed to save outbox event")
	}

	go func() {
		updateCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ms.userClientSvc.UpdateLastActivity(updateCtx, msg.UserID.String())
	}()

	return nil
}

func (ms *MessageService) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*message.BaseMessage, error) {
	if _, err := ms.roomSvc.GetRoom(ctx, roomID); err != nil {
		return nil, apperror.New(apperror.ErrRoomNotFound, "room not found")
	}

	return ms.storage.GetRoomMessages(ctx, roomID, limit)
}

func (ms *MessageService) GetMessage(ctx context.Context, id string) (*message.BaseMessage, error) {
	msg, err := ms.storage.GetMessage(ctx, id)
	if err != nil {
		return nil, apperror.New(apperror.ErrInternal, "failed to get message")
	}
	return msg, nil
}

func (ms *MessageService) SaveMessageBatch(ctx context.Context, msgs []*message.BaseMessage) error {
	return ms.storage.SaveMessageBatch(ctx, msgs)
}

func (ms *MessageService) BroadcastToRoom(ctx context.Context, msg *message.BaseMessage, roomID string) error {
	msg.RoomID = types.RoomIDFromString(roomID)
	return ms.distributor.Broadcast(ctx, msg)
}

func (ms *MessageService) SendToUserClient(ctx context.Context, userID string, msg *message.BaseMessage) error {
	msg.UserID = types.UserIDFromString(userID)
	return ms.distributor.Broadcast(ctx, msg)
}

internal/message/service/outboxWorker.go:
package service

import (
	"context"
	"encoding/json"
	message "krampus/internal/message/domain"
	"log"
	"time"
)

type OutboxPublisher interface {
	Publish(ctx context.Context, msg *message.BaseMessage) error
}

type MessageRepository interface {
	GetMessage(ctx context.Context, id string) (*message.BaseMessage, error)
}

type OutboxRepository interface {
	SaveEvent(
		ctx context.Context,
		event *message.OutboxEvent,
	) error

	GetUnpublishedEvents(
		ctx context.Context,
		limit int,
	) ([]*message.OutboxEvent, error)

	MarkPublished(
		ctx context.Context,
		eventID string,
	) error

	IncrementRetry(
		ctx context.Context,
		eventID string,
		errMsg string,
	) error
}

type OutboxWorker struct {
	repo        OutboxRepository
	messageRepo MessageRepository
	publisher   OutboxPublisher
}

func NewOutboxWorker(
	repo OutboxRepository,
	msgRepo MessageRepository,
	publisher OutboxPublisher,
) *OutboxWorker {

	return &OutboxWorker{
		repo:        repo,
		messageRepo: msgRepo,
		publisher:   publisher,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *OutboxWorker) process(ctx context.Context) {
	events, err := w.repo.GetUnpublishedEvents(ctx, 100)
	if err != nil {
		log.Printf("outbox load failed: %v", err)
		return
	}

	for _, event := range events {
		var eventPayload message.MessageCreatedEvent
		err := json.Unmarshal(event.Payload, &eventPayload)
		if err != nil {
			log.Printf("outbox payload decode failed event_id=%s err=%v", event.ID, err)
			_ = w.repo.IncrementRetry(ctx, event.ID, err.Error())
			continue
		}

		msg, err := w.messageRepo.GetMessage(ctx, eventPayload.MessageID.String())
		if err != nil {
			log.Printf("message load failed message_id=%s err=%v", eventPayload.MessageID, err)
			_ = w.repo.IncrementRetry(ctx, event.ID, err.Error())
			continue
		}

		err = w.publisher.Publish(ctx, msg)
		if err != nil {
			log.Printf("outbox publish failed event_id=%s message_id=%s err=%v", event.ID, msg.ID, err)
			_ = w.repo.IncrementRetry(ctx, event.ID, err.Error())
			continue
		}

		err = w.repo.MarkPublished(ctx, event.ID)
		if err != nil {
			log.Printf("outbox mark published failed event_id=%s err=%v", event.ID, err)
			continue
		}
	}
}

internal/message/service/rateLimiterService.go:
package service

import (
	"context"
	"krampus/internal/message/domain"
	"krampus/pkg/apperror"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	lastUsed map[string]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		lastUsed: make(map[string]time.Time),
	}
}

func (rl *RateLimiter) Check(ctx context.Context, userID string, msgType domain.MessageType) error {
	key := userID + ":" + string(msgType)

	rl.mu.Lock()
	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rl.createLimiter(msgType)
		rl.limiters[key] = limiter
	}
	rl.lastUsed[key] = time.Now()
	rl.mu.Unlock()

	if !limiter.Allow() {
		return apperror.New(apperror.ErrRateLimit, "rate exceeded")
	}

	return nil
}

func (rl *RateLimiter) createLimiter(msgType domain.MessageType) *rate.Limiter {
	switch msgType {
	case domain.TypeText:
		return rate.NewLimiter(rate.Limit(10), 10)
	case domain.TypeCommand:
		return rate.NewLimiter(rate.Limit(2), 2)
	case domain.TypeTyping:
		return rate.NewLimiter(rate.Limit(5), 5)
	case domain.TypeFile:
		return rate.NewLimiter(rate.Limit(1), 3)
	default:
		return rate.NewLimiter(rate.Limit(50), 50)
	}
}

func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for key := range rl.limiters {
		if time.Since(rl.lastUsed[key]) > 10*time.Minute {
			delete(rl.limiters, key)
			delete(rl.lastUsed, key)
		}
	}
}

internal/message/service/retryWorker.go:
package service

import (
	"context"
	"log"
	"math/rand"
	"time"

	"krampus/internal/message/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/types"
)

type RetryProcessor interface {
	ProcessDelivery(ctx context.Context, job *domain.DeliveryJob) error
}

type RetryRepository interface {
	Enqueue(ctx context.Context, job *domain.DeliveryJob) error
	GetReadyJobs(ctx context.Context, limit int) ([]*domain.DeliveryJob, error)
	Delete(ctx context.Context, messageID types.MessageID, userID types.UserID) error
}

type DLQRepository interface {
	Store(ctx context.Context, job *domain.DeliveryJob, reason string) error
}

type RetryWorker struct {
	repo         RetryRepository
	dlqRepo      DLQRepository
	processor    RetryProcessor
	pollInterval time.Duration
	maxAttempts  int
}

func NewRetryWorker(repo RetryRepository, dlqRepo DLQRepository, processor RetryProcessor) *RetryWorker {
	return &RetryWorker{
		repo:         repo,
		dlqRepo:      dlqRepo,
		processor:    processor,
		pollInterval: time.Second,
		maxAttempts:  5,
	}
}

func (w *RetryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *RetryWorker) processBatch(ctx context.Context) {
	jobs, err := w.repo.GetReadyJobs(ctx, 100)
	if err != nil {
		log.Printf("retry worker fetch error: %v", err)
		return
	}

	for _, job := range jobs {
		err := w.processor.ProcessDelivery(ctx, job)
		if err == nil {
			_ = w.repo.Delete(ctx, job.Message.ID, job.UserID)
			continue
		}

		if !apperror.IsRetryable(err) {
			storeErr := w.dlqRepo.Store(ctx, job, err.Error())
			if storeErr != nil {
				log.Printf("dlq store failed: %v", storeErr)
			}
			_ = w.repo.Delete(ctx, job.Message.ID, job.UserID)
			continue
		}

		job.Attempt++
		if job.Attempt >= w.maxAttempts {
			storeErr := w.dlqRepo.Store(ctx, job, "retry attempts exhausted")

			if storeErr != nil {
				log.Printf("dlq exhausted store failed: %v", storeErr)
			}
			_ = w.repo.Delete(ctx, job.Message.ID, job.UserID)
			continue
		}

		base := time.Second * time.Duration(1<<job.Attempt)
		jitter := time.Duration(rand.Int63n(int64(base / 2)))
		job.NextRetryAt = time.Now().Add(base + jitter)

		err = w.repo.Enqueue(ctx, job)
		if err != nil {
			log.Printf("retry re-enqueue failed: %v", err)
		}
	}
}

internal/message/storage/deliveryStatusRepository.go:
package storage

import (
	"context"
	"time"

	message "krampus/internal/message/domain"
	sqlc "krampus/internal/sqlc"
	"krampus/pkg/types"

	"github.com/jackc/pgx/v5/pgtype"
)

type PSQLDeliveryStatusRepository struct {
	queries *sqlc.Queries
}

func NewPSQLDeliveryStatusRepository(queries *sqlc.Queries) *PSQLDeliveryStatusRepository {
	return &PSQLDeliveryStatusRepository{
		queries: queries,
	}
}

func (r *PSQLDeliveryStatusRepository) UpdateStatus(ctx context.Context, messageID types.MessageID, userID types.UserID, status message.DeliveryStatus) error {
	return r.queries.UpsertDeliveryStatus(ctx, sqlc.UpsertDeliveryStatusParams{
		MessageID: messageID.String(),
		UserID:    userID.String(),
		Status:    string(status),
	},
	)
}

func (r *PSQLDeliveryStatusRepository) MarkDelivered(ctx context.Context, messageID types.MessageID, userID types.UserID, at time.Time) error {
	return r.queries.UpsertDeliveryStatus(ctx, sqlc.UpsertDeliveryStatusParams{
		MessageID: messageID.String(),
		UserID:    userID.String(),
		Status: string(
			message.DeliveryDelivered,
		),
		DeliveredAt: pgtype.Timestamp{
			Time:  at,
			Valid: true,
		},
	},
	)
}

func (r *PSQLDeliveryStatusRepository) MarkRead(ctx context.Context, messageID types.MessageID, userID types.UserID, at time.Time) error {
	return r.queries.UpsertDeliveryStatus(ctx, sqlc.UpsertDeliveryStatusParams{
		MessageID: messageID.String(),
		UserID:    userID.String(),
		Status:    string(message.DeliveryRead),
		ReadAt: pgtype.Timestamp{
			Time:  at,
			Valid: true,
		},
	},
	)
}

internal/message/storage/DLQStorage.go:
package storage

import (
	"context"
	"encoding/json"

	message "krampus/internal/message/domain"
	sqlc "krampus/internal/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type PSQLDLQRepository struct {
	queries *sqlc.Queries
}

func NewPSQLDLQRepository(queries *sqlc.Queries) *PSQLDLQRepository {
	return &PSQLDLQRepository{
		queries: queries,
	}
}

func (r *PSQLDLQRepository) Store(ctx context.Context, job *message.DeliveryJob, reason string) error {
	payload, err := json.Marshal(job.Message)
	if err != nil {
		return err
	}

	return r.queries.CreateDLQMessage(ctx, sqlc.CreateDLQMessageParams{
		ID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		MessageID: job.Message.ID.String(),
		UserID:    job.UserID.String(),
		RoomID:    job.RoomID.String(),
		Payload:   payload,
		Reason:    reason,
	},
	)
}

internal/message/storage/fileStorage.go:
package storage

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	chatDomain "krampus/internal/chat/domain"
	chat "krampus/internal/chat/service"
	"krampus/internal/message/domain"
)

type FileStorage struct {
	basePath    string
	segmentSize time.Duration
	buffers     sync.Map // roomID → *RoomBuffer
	roomSvc     *chat.RoomService
}

type RoomBuffer struct {
	mu         sync.Mutex
	messages   []*domain.BaseMessage
	size       int64
	lastFlush  time.Time
	activeFile *os.File
	writer     *bufio.Writer
}

func NewFileStorage(basePath string, segmentSize time.Duration, roomSvc *chat.RoomService) *FileStorage {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		log.Printf("Failed to create base directory: %v", err)
	}
	return &FileStorage{
		basePath:    basePath,
		segmentSize: segmentSize,
		roomSvc:     roomSvc,
	}
}

func (f *FileStorage) SaveMessage(roomID string, msg *domain.BaseMessage) error {
	buffer := f.getOrCreateBuffer(roomID)
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	buffer.messages = append(buffer.messages, msg)
	messageSize := int64(len(msg.Payload) + 100) // + метаданные

	// 🔥 УМНАЯ СТРАТЕГИЯ FLUSH
	shouldFlush := false
	switch msg.Type {
	case domain.TypeSystem, domain.TypeCommand:
		shouldFlush = true // немедленная запись

	case domain.TypeText, domain.TypeFile:
		buffer.size += messageSize
		shouldFlush = buffer.size >= 64*1024 || time.Since(buffer.lastFlush) > 100*time.Millisecond

	case domain.TypeTyping, domain.TypeReadReceipt:
		shouldFlush = time.Since(buffer.lastFlush) > 500*time.Millisecond

	default:
		shouldFlush = len(buffer.messages) >= 50
	}

	if shouldFlush {
		return f.flushBuffer(roomID, buffer)
	}
	return nil
}

func (f *FileStorage) getOrCreateBuffer(roomID string) *RoomBuffer {
	actual, _ := f.buffers.LoadOrStore(roomID, &RoomBuffer{
		messages:  make([]*domain.BaseMessage, 0),
		lastFlush: time.Now(),
	})
	return actual.(*RoomBuffer)
}

func (f *FileStorage) flushBuffer(roomID string, buffer *RoomBuffer) error {
	if len(buffer.messages) == 0 {
		return nil
	}

	if err := f.ensureFile(roomID, buffer); err != nil {
		return err
	}

	// 📝 Запись всех сообщений
	for _, msg := range buffer.messages {
		line := f.formatMessageLine(msg)
		if _, err := buffer.writer.WriteString(line); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}

	if err := buffer.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	if err := buffer.activeFile.Sync(); err != nil {
		log.Printf("Failed to sync file: %v", err)
	}

	// 🧹 Очистка буфера
	buffer.messages = buffer.messages[:0]
	buffer.size = 0
	buffer.lastFlush = time.Now()

	return nil
}

func (f *FileStorage) ensureFile(roomID string, buffer *RoomBuffer) error {
	now := time.Now()
	filePath := f.getSegmentPath(roomID, now)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if buffer.activeFile != nil {
		buffer.activeFile.Close()
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	buffer.activeFile = file
	buffer.writer = bufio.NewWriterSize(file, 64*1024) // 64KB буфер
	return nil
}

// 🗂️ УМНАЯ СЕГМЕНТАЦИЯ ПО ТИПАМ КОМНАТ
func (f *FileStorage) getSegmentPath(roomID string, t time.Time) string {
	roomType := f.getRoomType(roomID)

	switch roomType {
	case chatDomain.RoomVideoCall:
		// Видеозвонки: 1ч сегменты
		return filepath.Join(f.basePath, "video_calls", roomID,
			t.Format("2006-01-02"), fmt.Sprintf("%d.log", t.Hour()))

	case chatDomain.RoomGroup:
		// Групповые: 4ч сегменты
		hour := (t.Hour() / 4) * 4
		return filepath.Join(f.basePath, "groups", roomID,
			t.Format("2006-01"), fmt.Sprintf("%s_%02d.log", t.Format("2006-01-02"), hour))

	case chatDomain.RoomPrivate:
		// Личные: 1 день + шардинг
		shard := roomID[:2]
		return filepath.Join(f.basePath, "private", shard, roomID,
			t.Format("2006-01-02")+".log")

	case chatDomain.RoomPersonal:
		// Заметки: 1 месяц + шардинг
		shard := roomID[:2]
		return filepath.Join(f.basePath, "personal", shard, roomID,
			t.Format("2006-01")+".log")

	default:
		return filepath.Join(f.basePath, "default", roomID,
			t.Format("2006-01-02")+".log")
	}
}

func (f *FileStorage) formatMessageLine(msg *domain.BaseMessage) string {
	return fmt.Sprintf("%d|%s|%s|%s|%s|%s\n",
		msg.Timestamp, msg.ID, msg.Type, msg.UserID, msg.RoomID, string(msg.Payload))
}

func (f *FileStorage) getRoomType(roomID string) chatDomain.RoomType {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	room, err := f.roomSvc.GetRoom(ctx, roomID)
	if err != nil || room == nil {
		if len(roomID) > 0 {
			switch roomID[0] {
			case 'u':
				return chatDomain.RoomPrivate
			case 'v':
				return chatDomain.RoomVideoCall
			case 'p':
				return chatDomain.RoomPersonal
			}
		}
		return chatDomain.RoomGroup
	}

	return room.Type
}

internal/message/storage/idempotencyRepository.go:
package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type IdempotencyRepositoryPsql struct {
	db *pgxpool.Pool
}

func NewIdempotencyRepositoryPsql(
	db *pgxpool.Pool,
) *IdempotencyRepositoryPsql {

	return &IdempotencyRepositoryPsql{
		db: db,
	}
}

func (r *IdempotencyRepositoryPsql) IsDuplicate(
	ctx context.Context,
	key string,
) (bool, error) {

	var exists bool

	err := r.db.QueryRow(
		ctx,
		`
		SELECT EXISTS(
			SELECT 1
			FROM message_deduplication
			WHERE idempotency_key = $1
		)
		`,
		key,
	).Scan(&exists)

	return exists, err
}

func (r *IdempotencyRepositoryPsql) Save(
	ctx context.Context,
	key string,
	messageID string,
) error {

	_, err := r.db.Exec(
		ctx,
		`
		INSERT INTO message_deduplication (
			idempotency_key,
			message_id
		)
		VALUES ($1,$2)
		ON CONFLICT DO NOTHING
		`,
		key,
		messageID,
	)

	return err
}

internal/message/storage/MessageDistributor.go:
package storage

import (
	"context"
	"encoding/json"
	"strings"

	"krampus/internal/message/domain"
	"krampus/pkg/config"
	"krampus/pkg/logging"
	"krampus/pkg/types"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type MessageDistributor struct {
	producer *kafka.Producer
	logger   logging.Logger
	topic    string
	ctx      context.Context
	cancel   context.CancelFunc
}

type KafkaMessage struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	UserID    string          `json:"user_id"`
	RoomID    string          `json:"room_id"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

func NewMessageDistributor(cfg config.KafkaConfig, logger logging.Logger) *MessageDistributor {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(cfg.Brokers, ","),
		"client.id":          "krampus-msg-distributor",
		"acks":               "all",
		"enable.idempotence": true,
	})
	if err != nil {
		logger.Fatalf("Kafka producer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	d := &MessageDistributor{
		producer: p,
		logger:   logger,
		topic:    cfg.Topics.Incoming,
		ctx:      ctx,
		cancel:   cancel,
	}

	go d.handleDeliveryReports()

	return d
}

// BroadcastToRoom — отправка в конкретную комнату (реализация идентична Broadcast, но с подменой темы если нужно)
func (d *MessageDistributor) BroadcastToRoom(ctx context.Context, msg *domain.BaseMessage, roomID string) error {
	msg.RoomID = types.RoomIDFromString(roomID)
	return d.Broadcast(ctx, msg)
}

// SendToUserClient — отправка персонального сообщения (например, в топик уведомлений)
func (d *MessageDistributor) SendToUserClient(ctx context.Context, userID string, msg *domain.BaseMessage) error {
	msg.UserID = types.UserIDFromString(userID)
	return d.Broadcast(ctx, msg)
}

func (d *MessageDistributor) Broadcast(ctx context.Context, msg *domain.BaseMessage) error {
	event := KafkaMessage{
		ID:        msg.ID.String(),
		Type:      string(msg.Type),
		UserID:    msg.UserID.String(),
		RoomID:    msg.RoomID.String(),
		Timestamp: msg.Timestamp,
		Payload:   msg.Payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = d.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &d.topic,
			Partition: kafka.PartitionAny,
		},
		Value: data,
		Key:   []byte(msg.RoomID),
	}, nil)

	return nil
}

func (d *MessageDistributor) Publish(ctx context.Context, msg *domain.BaseMessage) error {
	return d.Broadcast(ctx, msg)
}

func (d *MessageDistributor) handleDeliveryReports() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case event := <-d.producer.Events():
			switch e := event.(type) {
			case *kafka.Message:
				if e.TopicPartition.Error != nil {
					d.logger.Errorf("kafka delivery failed topic=%s err=%v", *e.TopicPartition.Topic, e.TopicPartition.Error)
				} else {
					d.logger.Infof("kafka delivered topic=%s partition=%d offset=%v", *e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset)
				}
			}
		}
	}
}

func (d *MessageDistributor) Close() {
	d.cancel()
	d.producer.Flush(15 * 1000)
	d.producer.Close()
}

internal/message/storage/OutboxRepo.go:
package storage

import (
	"context"
	"encoding/json"
	"time"

	message "krampus/internal/message/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepositoryPsql struct {
	db *pgxpool.Pool
}

func NewOutboxRepositoryPsql(
	db *pgxpool.Pool,
) *OutboxRepositoryPsql {

	return &OutboxRepositoryPsql{
		db: db,
	}
}

func (r *OutboxRepositoryPsql) SaveEvent(
	ctx context.Context,
	event *message.OutboxEvent,
) error {

	_, err := r.db.Exec(
		ctx,
		`
		INSERT INTO outbox_events (
			id,
			aggregate_type,
			aggregate_id,
			event_type,
			payload,
			created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6)
		`,
		uuid.MustParse(event.ID),
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.Payload,
		time.Now(),
	)

	return err
}

func (r *OutboxRepositoryPsql) GetUnpublishedEvents(
	ctx context.Context,
	limit int,
) ([]*message.OutboxEvent, error) {

	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			id,
			aggregate_type,
			aggregate_id,
			event_type,
			payload,
			created_at,
			retry_count,
			COALESCE(last_error, '')
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
		`,
		limit,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var result []*message.OutboxEvent

	for rows.Next() {

		var event message.OutboxEvent

		var payload json.RawMessage

		err := rows.Scan(
			&event.ID,
			&event.AggregateType,
			&event.AggregateID,
			&event.EventType,
			&payload,
			&event.CreatedAt,
			&event.RetryCount,
			&event.LastError,
		)

		if err != nil {
			return nil, err
		}

		event.Payload = payload

		result = append(result, &event)
	}

	return result, nil
}

func (r *OutboxRepositoryPsql) MarkPublished(
	ctx context.Context,
	eventID string,
) error {

	_, err := r.db.Exec(
		ctx,
		`
		UPDATE outbox_events
		SET published_at = NOW()
		WHERE id = $1
		`,
		uuid.MustParse(eventID),
	)

	return err
}

func (r *OutboxRepositoryPsql) IncrementRetry(
	ctx context.Context,
	eventID string,
	erMsg string,
) error {

	_, err := r.db.Exec(
		ctx,
		`
		UPDATE outbox_events
		SET
			retry_count = retry_count + 1,
			last_error = $2
		WHERE id = $1
		`,
		uuid.MustParse(eventID),
		erMsg,
	)

	return err
}

internal/message/storage/psqlStorage.go:
package storage

import (
	"context"
	"fmt"

	"krampus/internal/message/domain"
	database "krampus/internal/sqlc"
	"krampus/pkg/types"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessagePGStorage struct {
	pool    *pgxpool.Pool
	queries *database.Queries
}

func NewMessagePGStorage(pool *pgxpool.Pool, queries *database.Queries) *MessagePGStorage {
	return &MessagePGStorage{
		pool:    pool,
		queries: queries,
	}
}

// SaveMessage — сохранение одного сообщения через sqlc
func (p *MessagePGStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
	return p.queries.SaveMessage(ctx, database.SaveMessageParams{
		ID:        msg.ID.String(),
		Type:      string(msg.Type),
		UserID:    msg.UserID.String(),
		RoomID:    msg.RoomID.String(),
		Timestamp: msg.Timestamp,
		Payload:   msg.Payload,
		Signature: pgtype.Text{String: msg.Signature, Valid: msg.Signature != ""},
	})
}

// SaveMessageBatch — пакетное сохранение через транзакцию sqlc
func (p *MessagePGStorage) SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error {
	// Создаем транзакцию, используя переданный пул
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// "Привязываем" запросы к этой транзакции
	qtx := p.queries.WithTx(tx)

	for _, msg := range msgs {
		err := qtx.SaveMessage(ctx, database.SaveMessageParams{
			ID:        msg.ID.String(),
			Type:      string(msg.Type),
			UserID:    msg.UserID.String(),
			RoomID:    msg.RoomID.String(),
			Timestamp: msg.Timestamp,
			Payload:   msg.Payload,
			Signature: pgtype.Text{String: msg.Signature, Valid: msg.Signature != ""},
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetRoomMessages — история чата через sqlc
func (p *MessagePGStorage) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error) {
	rows, err := p.queries.GetRoomMessages(ctx, database.GetRoomMessagesParams{
		RoomID: roomID,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get room messages: %w", err)
	}

	messages := make([]*domain.BaseMessage, 0, len(rows))
	for _, row := range rows {
		var ts int64
		if row.CreatedAt.Valid {
			ts = row.CreatedAt.Time.UnixNano()
		}

		messages = append(messages, &domain.BaseMessage{
			ID:        types.MessageID(row.ID),
			Type:      domain.MessageType(row.Type),
			UserID:    types.UserID(row.UserID),
			RoomID:    types.RoomID(row.RoomID),
			Timestamp: ts,
			Payload:   row.Payload,
			Signature: row.Signature.String,
		})
	}

	// Реверс для хронологического порядка
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetMessage — получение одного сообщения по ID
func (p *MessagePGStorage) GetMessage(ctx context.Context, id string) (*domain.BaseMessage, error) {
	row, err := p.queries.GetMessage(ctx, id)
	if err != nil {
		return nil, err
	}

	var ts int64
	if row.CreatedAt.Valid {
		ts = row.CreatedAt.Time.UnixNano()
	}

	return &domain.BaseMessage{
		ID:        types.MessageID(row.ID),
		Type:      domain.MessageType(row.Type),
		UserID:    types.UserID(row.UserID),
		RoomID:    types.RoomID(row.RoomID),
		Timestamp: ts,
		Payload:   row.Payload,
		Signature: row.Signature.String,
	}, nil
}

// CleanupOldMessages — удаление старых сообщений каждые 24ч
func (p *MessagePGStorage) CleanupOldMessages(ctx context.Context) error {
	return p.queries.CleanupOldMessages(ctx)
}

func (p *MessagePGStorage) Close() {
	p.pool.Close()
}

internal/message/storage/redisStorage.go:
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	chatDomain "krampus/internal/chat/domain"
	"krampus/internal/message/domain"
	userDomain "krampus/internal/user/domain"

	"github.com/redis/go-redis/v9"
)

type RedisStorage struct {
	client *redis.Client
}

func NewRedisStorage(addr, password string, db int) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{client: client}, nil
}

// CacheRecentMessage — LRU 100 последних (pipeline)
func (r *RedisStorage) CacheRecentMessage(ctx context.Context, roomID string, msg *domain.BaseMessage) error {
	key := "recent_messages:" + roomID
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	pipe := r.client.Pipeline()
	pipe.LPush(ctx, key, msgJSON)
	pipe.LTrim(ctx, key, 0, 99) // топ 100
	pipe.Expire(ctx, key, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	return err
}

// GetRecentMessages — быстрый кэш для новых клиентов
func (r *RedisStorage) GetRecentMessages(ctx context.Context, roomID string) ([]*domain.BaseMessage, error) {
	key := "recent_messages:" + roomID
	msgsJSON, err := r.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get cached messages: %w", err)
	}

	var messages []*domain.BaseMessage
	for _, msgJSON := range msgsJSON {
		var msg domain.BaseMessage
		if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
			continue // пропускаем битые
		}
		messages = append(messages, &msg)
	}
	return messages, nil
}

// 👥 User Connections (активные сессии)
func (r *RedisStorage) SetUserConnection(ctx context.Context, userID string, conn *chatDomain.UserConnection) error {
	key := "user_connections:" + userID
	connJSON, err := json.Marshal(conn)
	if err != nil {
		return fmt.Errorf("failed to marshal connection: %w", err)
	}

	if err := r.client.HSet(ctx, key, conn.ConnID, connJSON).Err(); err != nil {
		return fmt.Errorf("failed to set user connection: %w", err)
	}
	return r.client.Expire(ctx, key, 30*time.Minute).Err()
}

func (r *RedisStorage) RemoveUserConnection(ctx context.Context, userID, connID string) error {
	key := "user_connections:" + userID
	return r.client.HDel(ctx, key, connID).Err()
}

// Room/User Cache
func (r *RedisStorage) GetRoom(ctx context.Context, id string) (*chatDomain.Room, error) {
	key := "room:" + id
	roomJSON, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room from cache: %w", err)
	}

	var room chatDomain.Room
	if err := json.Unmarshal([]byte(roomJSON), &room); err != nil {
		return nil, fmt.Errorf("failed to unmarshal room: %w", err)
	}
	return &room, nil
}

func (r *RedisStorage) SetRoom(ctx context.Context, id string, room *chatDomain.Room) error {
	key := "room:" + id
	roomJSON, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("failed to marshal room: %w", err)
	}
	return r.client.Set(ctx, key, roomJSON, 10*time.Minute).Err()
}

func (r *RedisStorage) GetAccessToken(token string) (*userDomain.CachedSession, error) {
	key := "access_token:" + token
	data, err := r.client.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}

	var session userDomain.CachedSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *RedisStorage) SetAccessToken(token string, session userDomain.CachedSession) error {
	key := "access_token:" + token
	data, _ := json.Marshal(session)
	expiration := time.Until(session.ExpiresAt)
	return r.client.Set(context.Background(), key, data, expiration).Err()
}

internal/message/storage/replayRepository.go:
package storage

import (
	"context"

	message "krampus/internal/message/domain"
	database "krampus/internal/sqlc"
	"krampus/pkg/types"
)

type PSQLReplayRepository struct {
	queries *database.Queries
}

func NewPSQLReplayRepository(
	queries *database.Queries,
) *PSQLReplayRepository {

	return &PSQLReplayRepository{
		queries: queries,
	}
}

func (r *PSQLReplayRepository) GetMessagesAfter(
	ctx context.Context,
	roomID types.RoomID,
	after int64,
	limit int,
) ([]*message.BaseMessage, error) {

	rows, err := r.queries.GetMessagesAfter(
		ctx,
		database.GetMessagesAfterParams{
			RoomID:     roomID.String(),
			AfterTs:    after,
			LimitCount: int32(limit),
		},
	)

	if err != nil {
		return nil, err
	}

	result := make([]*message.BaseMessage, 0, len(rows))

	for _, row := range rows {

		result = append(result, &message.BaseMessage{
			ID:        types.MessageID(row.ID),
			Type:      message.MessageType(row.Type),
			UserID:    types.UserID(row.UserID),
			RoomID:    types.RoomID(row.RoomID),
			Timestamp: row.Timestamp,
			Payload:   row.Payload,
			Signature: func() string {
				if row.Signature.Valid {
					return row.Signature.String
				}
				return ""
			}(),
		})
	}

	return result, nil
}

internal/message/storage/retryStorage.go:
package storage

import (
	"context"
	"encoding/json"

	message "krampus/internal/message/domain"
	database "krampus/internal/sqlc"
	"krampus/pkg/types"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type PSQLRetryRepository struct {
	queries *database.Queries
}

func NewPSQLRetryRepository(queries *database.Queries) *PSQLRetryRepository {
	return &PSQLRetryRepository{
		queries: queries,
	}
}

func (r *PSQLRetryRepository) Enqueue(ctx context.Context, job *message.DeliveryJob) error {
	payload, err := json.Marshal(job.Message)
	if err != nil {
		return err
	}

	return r.queries.CreateRetryJob(ctx, database.CreateRetryJobParams{
		ID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		MessageID: job.Message.ID.String(),
		UserID:    job.UserID.String(),
		RoomID:    job.RoomID.String(),
		Payload:   payload,
		Attempt:   int32(job.Attempt),
		NextRetryAt: pgtype.Timestamp{
			Time:  job.NextRetryAt,
			Valid: true,
		},
	},
	)
}

func (r *PSQLRetryRepository) GetReadyJobs(ctx context.Context, limit int) ([]*message.DeliveryJob, error) {
	rows, err := r.queries.GetReadyRetryJobs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}

	jobs := make([]*message.DeliveryJob, 0, len(rows))
	for _, row := range rows {
		var msg message.BaseMessage
		if err := json.Unmarshal(row.Payload, &msg); err != nil {
			continue
		}

		jobs = append(jobs, &message.DeliveryJob{
			Message:     &msg,
			UserID:      types.UserID(row.UserID),
			RoomID:      types.RoomID(row.RoomID),
			Attempt:     int(row.Attempt),
			NextRetryAt: row.NextRetryAt.Time,
		},
		)
	}

	return jobs, nil
}

func (r *PSQLRetryRepository) Delete(ctx context.Context, messageID types.MessageID, userID types.UserID) error {
	return r.queries.DeleteRetryJob(ctx, database.DeleteRetryJobParams{
		MessageID: messageID.String(),
		UserID:    userID.String(),
	},
	)
}

internal/user/adapters/refreshTokenHandler.go:
package adapters

import (
	"krampus/internal/user/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RefreshTokenService interface {
	// SaveSession() ()
	UserRefresh(token string) (domain.TokenResponse, error)
	// UserSendEmailCode(tempToken string) error
}

type refreshTokenHandler struct {
	refreshTokenService RefreshTokenService
	logger              *logging.Logger
}

func NewRefreshTokenHandler(s RefreshTokenService, l *logging.Logger) *refreshTokenHandler {
	return &refreshTokenHandler{
		refreshTokenService: s,
		logger:              l,
	}
}

func (h *refreshTokenHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/refresh", h.refresh)
	// rg.POST("/send-code", h.sendEmailToken) // раскомментируй, когда сервис будет готов
}

func (h *refreshTokenHandler) refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	token, err := h.refreshTokenService.UserRefresh(req.RefreshToken)
	if err != nil {
		h.logger.Error("Failed to refresh user: " + err.Error())
		appErr, ok := err.(*apperror.AppError)
		if ok {
			c.JSON(http.StatusUnauthorized, appErr)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Failed to refresh token",
			})
		}
		return
	}

	c.JSON(http.StatusOK, token)
}

// func (h *refreshTokenHandler) sendEmailToken(c *gin.Context) {
// 	var req struct {
// 		TempToken string `json:"temp_token"`
// 	}
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		h.logger.Error("Failed to bind JSON: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Invalid request body",
// 		})
// 		return
// 	}

// 	err := h.refreshTokenService.UserSendEmailCode(req.TempToken)
// 	if err != nil {
// 		h.logger.Error("Failed to send verification code: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Failed to send verification code",
// 		})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"success": true,
// 	})
// }

internal/user/adapters/userHandler.go:
package adapters

import (
	"krampus/internal/user/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/logging"
	"net/http"

	twofa "krampus/internal/auth/domain"

	"github.com/gin-gonic/gin"
)

type UserService interface {
	UserRegister(users domain.User) (domain.User, error)
	UserLogin(users domain.User) (domain.TokenResponse, twofa.TwoFaCodes, error)
	UserLogout(userID int64, password string) error
}

type userHandler struct {
	userService UserService
	logger      *logging.Logger
}

func NewUserHandler(s UserService, l *logging.Logger) *userHandler {
	return &userHandler{
		userService: s,
		logger:      l,
	}
}

func (h *userHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/signup", h.signUp)
	rg.POST("/signin", h.signIn)
	rg.POST("/logout", h.logout)
}

func (h *userHandler) signUp(c *gin.Context) {
	var user domain.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	createdUser, err := h.userService.UserRegister(user)
	if err != nil {
		h.logger.Error("Failed to register user: " + err.Error())
		appErr, ok := err.(*apperror.AppError)
		if ok {
			c.JSON(http.StatusBadRequest, appErr)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to register user",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, createdUser)
}

func (h *userHandler) signIn(c *gin.Context) {
	var user domain.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	accessToken, tempToken, err := h.userService.UserLogin(user)
	if err != nil {
		h.logger.Error("Failed to login user: " + err.Error())
		appErr, ok := err.(*apperror.AppError)
		if ok {
			c.JSON(http.StatusUnauthorized, appErr)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication failed",
			})
		}
		return
	}

	if tempToken.RequiresTwoFa {
		c.JSON(http.StatusOK, tempToken)
		return
	}

	c.JSON(http.StatusOK, accessToken)
}

func (h *userHandler) logout(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	var req domain.TwoFaToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	err := h.userService.UserLogout(userID.(int64), req.Password)
	if err != nil {
		h.logger.Error("Failed to logout: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to logout",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func handleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleMetrics() gin.HandlerFunc {
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

internal/user/domain/session.go:
package domain

import "time"

type CachedSession struct {
	UserID       int64     `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type CachedTempToken struct {
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

internal/user/domain/user.go:
package domain

import "time"

type ChatUserStatus string

const (
	StatusOnline  ChatUserStatus = "online"
	StatusAway    ChatUserStatus = "away"
	StatusOffline ChatUserStatus = "offline"
	StatusDND     ChatUserStatus = "dnd"
)

type User struct {
	ID           int64                  `json:"id"`
	Username     string                 `json:"username"`
	Firstname    string                 `json:"firstname"`
	Lastname     string                 `json:"lastname"`
	Email        string                 `json:"email"`
	Password     string                 `json:"password"`
	PasswordHash string                 `json:"-"`
	TwoFAEnabled bool                   `json:"two_fa_enabled"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActive   int64                  `json:"last_active"`
	Status       ChatUserStatus         `json:"status"`
	Permissions  []string               `json:"permissions"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	// BlockedUntil
	// FailedLogins
}

type Code struct {
	TempToken string `json:"temp_token"`
	Code      string `json:"code"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type TwoFaToggleRequest struct {
	Password string `json:"password"`
}

internal/user/service/refreshTokenService.go:
package service

import (
	"errors"
	"time"

	"krampus/internal/user/domain"

	"github.com/golang-jwt/jwt/v5"
)

type RefreshTokenStorage interface {
	RefreshStore(userID int64, token string, expiresAt time.Time) error
	RefreshGet(token string) (int64, error)
	RefreshDelete(token string) error
	RefreshDeleteByUserID(userID int64) error
}

type RefreshToken struct {
	refreshTokenStorage RefreshTokenStorage
	jwtSecret           string
}

func NewRefreshToken(refreshToken RefreshTokenStorage, jwt string) *RefreshToken {
	return &RefreshToken{
		refreshTokenStorage: refreshToken,
		jwtSecret:           jwt,
	}
}

func (s *RefreshToken) GenerateRefreshToken(id int64) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = id
	claims["exp"] = time.Now().Add(7 * 24 * time.Hour).Unix()

	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	err = s.refreshTokenStorage.RefreshStore(id, signed, expiresAt)
	return signed, err
}

func (s *RefreshToken) UserRefresh(refreshToken string) (domain.TokenResponse, error) {
	userID, err := s.refreshTokenStorage.RefreshGet(refreshToken)
	if err != nil {
		return domain.TokenResponse{}, errors.New("Invalid refresh token")
	}

	accessToken, err := s.GenerateAccessToken(userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	newRefreshToken, err := s.GenerateRefreshToken(userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	s.refreshTokenStorage.RefreshDelete(refreshToken)

	return domain.TokenResponse{AccessToken: accessToken, RefreshToken: newRefreshToken}, nil
}

func (s *RefreshToken) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *RefreshToken) DeleteRefreshTokensByUserID(userID int64) error {
	return s.refreshTokenStorage.RefreshDeleteByUserID(userID)
}

internal/user/service/userService.go:
package service

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"

	twofa "krampus/internal/auth/domain"
	"krampus/internal/user/domain"
	redis "krampus/internal/user/storage"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserStorage interface {
	InsertUser(user domain.User) (int64, error)
	SelectUserByEmail(email string) (domain.User, error)
	SelectUserByID(userID int64) (domain.User, error)
	BlockUser(email, blockedUntil string) error
	// RedisSessionStorage() redis.SessionStorage
}

type LoginAttemptStorage interface {
	LogAttempt(email string, result bool, attemptTime time.Time) error
	GetFailedLogAttempts(email string, windowStart time.Time) (int64, error)
	UserBlocked(email string, windowStart time.Time) ([]map[string]interface{}, error)
}

type User struct {
	userStorage         UserStorage
	loginAttemptStorage LoginAttemptStorage
	refreshTokenService *RefreshToken
	redisStorage        *redis.RedisSessionStorage
	jwtSecret           string
}

func NewUser(
	user UserStorage,
	loginAttempt LoginAttemptStorage,
	refreshToken *RefreshToken,
	redisStorage *redis.RedisSessionStorage,
	jwt string,
) *User {
	return &User{
		userStorage:         user,
		loginAttemptStorage: loginAttempt,
		refreshTokenService: refreshToken,
		redisStorage:        redisStorage,
		jwtSecret:           jwt,
	}
}

func (s *User) UserRegister(user domain.User) (domain.User, error) {
	fmt.Printf("DEBUG SERVICE REGISTER: Starting registration for: %s\n", user.Email)

	if user.Username == "" || user.Firstname == "" || user.Lastname == "" || user.Email == "" {
		return domain.User{}, errors.New("Invalid input: all fields are required")
	}

	if user.Password == "" || len(user.Password) < 8 {
		return domain.User{}, errors.New("Invalid password input: password must br at least 8 characters")
	}

	hasLetters, _ := regexp.MatchString(`[a-zA-Zа-яА-Я]`, user.Password)
	hasDigits, _ := regexp.MatchString(`[0-9]`, user.Password)
	hasSpecial, _ := regexp.MatchString(`[^a-zA-Zа-яА-Я0-9\s]`, user.Password)

	if !hasLetters || !hasDigits || !hasSpecial {
		return domain.User{}, errors.New("Invalid password input: password must contain letters, digits and special characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, errors.New("Error hashing password")
	}

	userToSave := domain.User{
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: string(hash),
		TwoFAEnabled: user.TwoFAEnabled,
	}

	fmt.Printf("DEBUG SERVICE REGISTER: Calling storage.InsertUser\n")
	id, err := s.userStorage.InsertUser(userToSave)
	if err != nil {
		fmt.Printf("DEBUG SERVICE REGISTER: Storage error: %v\n", err)
		return domain.User{}, err
	}

	createdUser := domain.User{
		ID:           id,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFAEnabled,
		CreatedAt:    time.Now(),
	}

	fmt.Printf("DEBUG SERVICE REGISTER: SUCCESS - Created student with ID: %d\n", id)
	return createdUser, nil
}

func (s *User) UserLogin(user domain.User) (domain.TokenResponse, twofa.TwoFaCodes, error) {
	fmt.Printf("DEBUG LOGIN: Attempting login for email: '%s'\n", user.Email)
	fmt.Printf("DEBUG LOGIN: Password provided: '%s'\n", user.Password)
	fmt.Printf("DEBUG LOGIN: TwoFA enabled: '%v'\n", user.TwoFAEnabled)

	if user.Email == "" || user.Password == "" {
		fmt.Printf("DEBUG LOGIN: Email or password empty\n")
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("email and password are required")
	}

	blocked, minutesLeft, err := s.IsUserBlocked(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error checking block status: %v\n", err)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	if blocked {
		fmt.Printf("DEBUG LOGIN: User is blocked for %d minutes\n", minutesLeft)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, fmt.Errorf("your account is blocked for %d minutes", minutesLeft)
	}

	fmt.Printf("DEBUG LOGIN: Searching user in database...\n")
	dbUser, err := s.userStorage.SelectUserByEmail(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Database error or user not found: %v\n", err)
		s.LogLoginAttempt(user.Email, false)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGIN: User found - ID: %d, Email: %s\n", dbUser.ID, dbUser.Email)
	fmt.Printf("DEBUG LOGIN: Stored password hash: %s\n", dbUser.PasswordHash)
	fmt.Printf("DEBUG LOGIN: Provided password: %s\n", user.Password)

	fmt.Printf("DEBUG LOGIN: Comparing passwords...\n")
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(user.Password))
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Password comparison failed: %v\n", err)
		s.LogLoginAttempt(user.Email, false)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGIN: Password correct!\n")

	attempts, err := s.GetFailedAttempts(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error getting failed attempts: %v\n", err)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	maxAttempts := int64(5)
	if attempts >= maxAttempts {
		fmt.Printf("DEBUG LOGIN: Too many failed attempts: %d\n", attempts)
		s.BlockUser(user.Email)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("too many failed attempts, account blocked")
	}

	// if dbUser.TwoFAEnabled {
	if dbUser.TwoFAEnabled != false {
		tempToken, err := s.GenerateTempToken(dbUser.ID)
		if err != nil {
			fmt.Printf("DEBUG LOGIN: Error generating temp token: %v\n", err)
			return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
		}
		return domain.TokenResponse{}, twofa.TwoFaCodes{RequiresTwoFa: true, TempToken: tempToken}, nil
	}

	accessToken, err := s.GenerateAccessToken(dbUser.ID)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error generating access token: %v\n", err)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	refreshToken, err := s.refreshTokenService.GenerateRefreshToken(dbUser.ID)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error generating refresh token: %v\n", err)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	session := domain.CachedSession{
		UserID:       dbUser.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	if err = s.redisStorage.SetAccessToken(accessToken, session); err != nil {
		fmt.Printf("Cache set error: %v\n", err)
	}
	if err = s.redisStorage.SetSessionByUserID(dbUser.ID, session); err != nil {
		fmt.Printf("Session cache error: %v\n", err)
	}

	s.LogLoginAttempt(user.Email, true)
	fmt.Printf("DEBUG LOGIN: Login successful for user ID: %d\n", dbUser.ID)
	return domain.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}, twofa.TwoFaCodes{}, nil
}

func (s *User) UserLogout(userID int64, password string) error {
	fmt.Printf("DEBUG LOGOUT: Searching user in database...\n")
	dbUser, err := s.userStorage.SelectUserByID(userID)
	if err != nil {
		fmt.Printf("DEBUG LOGOUT: Database error or user not found: %v\n", err)
		s.LogLoginAttempt(dbUser.Email, false)
		return errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGOUT: User found - ID: %d, Email: %s\n", dbUser.ID, dbUser.Email)
	fmt.Printf("DEBUG LOGOUT: Stored password hash: %s\n", dbUser.PasswordHash)
	fmt.Printf("DEBUG LOGOUT: Provided password: %s\n", dbUser.Password)

	fmt.Printf("DEBUG LOGOUT: Comparing passwords...\n")
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(dbUser.Password))
	if err != nil {
		fmt.Printf("DEBUG LOGOUT: Password comparison failed: %v\n", err)
		s.LogLoginAttempt(dbUser.Email, false)
		return errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGOUT: Password correct!\n")

	if err = s.redisStorage.DeleteSessionByUserID(userID); err != nil {
		fmt.Printf("Redis session delete error: %v\n", err)
	}

	err = s.refreshTokenService.DeleteRefreshTokensByUserID(userID)
	if err != nil {
		return err
	}

	fmt.Printf("DEBUG LOGOUT: User %d successful logged out\n", userID)
	return nil
}

func (s *User) BlockUser(email string) {
	now := time.Now()
	blockedUntil := now.Add(1 * time.Minute).Format(time.RFC3339)

	s.LogLoginAttempt(email, false)

	err := s.userStorage.BlockUser(email, blockedUntil)
	if err != nil {
		fmt.Printf("Ошибка блокировки: %v\n", err)
	}
}

func (s *User) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *User) GenerateTempToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(10 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *User) LogLoginAttempt(email string, result bool) {
	attemptTime := time.Now().UTC()

	err := s.loginAttemptStorage.LogAttempt(email, result, attemptTime)
	if err != nil {
		fmt.Printf("Ошибка логирования: %v\n", err)
	}
}

func (s *User) GetFailedAttempts(email string) (int64, error) {
	now := time.Now().UTC()
	windowStart := now.Add(-1 * time.Minute)

	count, err := s.loginAttemptStorage.GetFailedLogAttempts(email, windowStart)
	if err != nil {
		fmt.Printf("Ошибка подсчета попыток: %v\n", err)
		return int64(0), err
	}

	return int64(count), err
}

func (s *User) IsUserBlocked(email string) (bool, int64, error) {
	now := time.Now().UTC()
	windowStart := now

	result, err := s.loginAttemptStorage.UserBlocked(email, windowStart)
	if err != nil {
		fmt.Printf("Ошибка проверки блокировки: %v\n", err)
		return false, 0, err
	}

	if len(result) > 0 {
		blockedUntilStr, ok := result[0]["blocked_until"].(string)
		if !ok {
			return false, 0, errors.New("invalid format for blocked_until")
		}

		blockedUntil, err := time.Parse(time.RFC3339, blockedUntilStr)
		if err != nil {
			return false, 0, err
		}

		minutesLeft := math.Ceil(time.Until(blockedUntil).Minutes())
		if minutesLeft < 0 {
			minutesLeft = 0
		}

		return true, int64(minutesLeft), nil
	}

	return false, 0, nil
}

internal/user/storage/loginAttempt_storage.go:
package storage

import (
	"context"
	"time"

	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type LoginAttemptStorageError struct {
	Message string
}

type LoginAttemptStorage struct {
	queries *database.Queries
}

var (
	ErrLoginAttemptExpired = &LoginAttemptStorageError{"token expired"}
)

func NewLoginAttemptStorage(queries *database.Queries) *LoginAttemptStorage {
	return &LoginAttemptStorage{queries: queries}
}

func (e *LoginAttemptStorageError) LoginAttemptError() string {
	return e.Message
}

func (s *LoginAttemptStorage) LogAttempt(email string, result bool, attemptTime time.Time) error {
	params := database.CreateLoginAttemptParams{
		Email:       email,
		Success:     result,
		AttemptedAt: pgtype.Timestamptz{Time: attemptTime, Valid: true},
	}

	return s.queries.CreateLoginAttempt(context.Background(), params)
}

func (s *LoginAttemptStorage) GetFailedLogAttempts(email string, windowStart time.Time) (int64, error) {
	count, err := s.queries.GetRecentFailedAttempts(context.Background(), database.GetRecentFailedAttemptsParams{
		Email:       email,
		AttemptedAt: pgtype.Timestamptz{Time: windowStart, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *LoginAttemptStorage) UserBlocked(email string, windowStart time.Time) ([]map[string]interface{}, error) {
	blockedUntil, err := s.queries.GetBlockedStatus(context.Background(), email)
	if err != nil {
		return []map[string]interface{}{}, nil
	}

	var isBlocked bool
	if blockedUntil.Valid && blockedUntil.Time.After(time.Now()) {
		isBlocked = true
	}

	return []map[string]interface{}{
		{
			"blocked_until": blockedUntil.Time, // Нет такого поля в структуре
			"is_blocked":    isBlocked,
		},
	}, nil
}

internal/user/storage/psql_user_storage.go:
package storage

import (
	"context"
	"time"

	database "krampus/internal/sqlc"
	"krampus/internal/user/domain"

	"github.com/jackc/pgx/v5/pgtype"
)

type UserStorageError struct {
	Message string
}

type UserStorage struct {
	queries *database.Queries
}

var (
	ErrTokenExpired = &UserStorageError{"token expired"}
)

func NewUserStorage(queries *database.Queries) *UserStorage {
	return &UserStorage{queries: queries}
}

func (e *UserStorageError) UserError() string {
	return e.Message
}

func (s *UserStorage) InsertUser(user domain.User) (int64, error) {
	params := database.CreateUserParams{
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFaEnabled: pgtype.Bool{Bool: user.TwoFAEnabled, Valid: true},
	}

	createdUser, err := s.queries.CreateUser(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return createdUser.ID, nil
}

func (s *UserStorage) SelectUserByEmail(email string) (domain.User, error) {
	user, err := s.queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return domain.User{}, err
	}

	return domain.User{
		ID:           user.ID,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFaEnabled.Bool,
		CreatedAt:    user.CreatedAt.Time,
	}, nil
}

func (s *UserStorage) SelectUserByID(id int64) (domain.User, error) {
	user, err := s.queries.GetUserByID(context.Background(), id)
	if err != nil {
		return domain.User{}, nil
	}

	return domain.User{
		ID:           user.ID,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFaEnabled.Bool,
		CreatedAt:    user.CreatedAt.Time,
	}, nil
}

func (s *UserStorage) BlockUser(email, blockedUntil string) error {
	var blockedUntilTime pgtype.Timestamptz
	if blockedUntil != "" {
		t, err := time.Parse(time.RFC3339, blockedUntil)
		if err != nil {
			return err
		}
		blockedUntilTime = pgtype.Timestamptz{Time: t, Valid: true}
	} else {
		blockedUntilTime = pgtype.Timestamptz{Valid: false}
	}

	return s.queries.BlockUser(context.Background(), database.BlockUserParams{
		Email:        email,
		BlockedUntil: blockedUntilTime,
	})
}

internal/user/storage/redis_session_storage.go:
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"krampus/internal/user/domain"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisSessionStorage struct {
	client *redis.Client
}

func NewRedisSessionStorage(client *redis.Client) *RedisSessionStorage {
	return &RedisSessionStorage{
		client: client,
	}
}

func (s *RedisSessionStorage) SetAccessToken(token string, session domain.CachedSession) error {
	key := fmt.Sprintf("access: %s", token)

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	return s.client.Set(context.Background(), key, data, ttl).Err()
}

func (s *RedisSessionStorage) GetAccessToken(token string) (domain.CachedSession, error) {
	key := fmt.Sprintf("access: %s", token)

	data, err := s.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return domain.CachedSession{}, fmt.Errorf("token not found")
	}
	if err != nil {
		return domain.CachedSession{}, fmt.Errorf("redis get: %w", err)
	}

	var session domain.CachedSession
	if err = json.Unmarshal([]byte(data), &session); err != nil {
		return domain.CachedSession{}, fmt.Errorf("json unmarshal: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		s.client.Del(context.Background(), key)
		return domain.CachedSession{}, fmt.Errorf("token expired")
	}

	return session, nil
}

func (s *RedisSessionStorage) DeleteAccessToken(token string) error {
	key := fmt.Sprintf("access: %s", token)
	return s.client.Del(context.Background(), key).Err()
}

func (s *RedisSessionStorage) GetSessionByUserID(userID int64) (domain.CachedTempToken, error) {
	key := fmt.Sprintf("session:%d", userID)
	data, err := s.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return domain.CachedTempToken{}, fmt.Errorf("temp token not found")
	}
	if err != nil {
		return domain.CachedTempToken{}, err
	}

	var tempToken domain.CachedTempToken
	json.Unmarshal([]byte(data), &tempToken)
	return tempToken, nil
}

func (s *RedisSessionStorage) DeleteSessionByUserID(userID int64) error {
	key := fmt.Sprintf("session:%d", userID)
	return s.client.Del(context.Background(), key).Err()
}

func (s *RedisSessionStorage) SetSessionByUserID(userID int64, session domain.CachedSession) error {
	key := fmt.Sprintf("session:%d", userID)
	data, _ := json.Marshal(session)
	return s.client.Set(context.Background(), key, data, 7*24*time.Hour).Err()
}

func (s *RedisSessionStorage) SetTempToken(token string, temp domain.CachedTempToken) error {
	key := fmt.Sprintf("temp:%s", token)
	data, _ := json.Marshal(temp)
	ttl := time.Until(temp.ExpiresAt)
	return s.client.Set(context.Background(), key, data, ttl).Err()
}

func (s *RedisSessionStorage) GetTempToken(token string) (domain.CachedTempToken, error) {
	key := fmt.Sprintf("temp:%s", token)
	data, err := s.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return domain.CachedTempToken{}, fmt.Errorf("temp token not found")
	}
	if err != nil {
		return domain.CachedTempToken{}, fmt.Errorf("redis get temp: %w", err)
	}

	var tempToken domain.CachedTempToken
	if err := json.Unmarshal([]byte(data), &tempToken); err != nil {
		return domain.CachedTempToken{}, fmt.Errorf("json unmarshal temp: %w", err)
	}

	return tempToken, nil
}

func (r *RedisSessionStorage) ValidateSession(ctx context.Context, sessionID string, userID string) error {
	session, err := r.GetAccessToken(sessionID)

	if err != nil {
		return err
	}

	if session.UserID == 0 {
		return errors.New("session not found")
	}

	if strconv.FormatInt(session.UserID, 10) != userID {
		return errors.New("invalid session owner")
	}

	return nil
}

func (s *RedisSessionStorage) RedisSessionStorage() {
	// Оставляем пустым, если реальная логика не требуется,
	// но метод нужен для реализации интерфейса
}

internal/user/storage/refreshToken_storage.go:
package storage

import (
	"context"
	"errors"
	"time"

	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type RefreshTokenStorageError struct {
	Message string
}

type RefreshTokenStorage struct {
	queries *database.Queries
}

var (
	ErrRefreshTokenExpired = errors.New("token expired")
)

func NewRefreshTokenStorage(queries *database.Queries) *RefreshTokenStorage {
	return &RefreshTokenStorage{queries: queries}
}

func (e *RefreshTokenStorageError) RefreshTokenError() string {
	return e.Message
}

func (s *RefreshTokenStorage) RefreshStore(userID int64, token string, expiresAt time.Time) error {
	params := database.CreateRefreshTokenParams{
		UserID:    userID,
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}

	return s.queries.CreateRefreshToken(context.Background(), params)
}

func (s *RefreshTokenStorage) RefreshGet(token string) (int64, error) {
	refreshToken, err := s.queries.GetRefreshToken(context.Background(), token)
	if err != nil {
		return 0, err
	}

	if time.Now().After(refreshToken.ExpiresAt.Time) {
		s.RefreshDelete(token)
		return 0, ErrRefreshTokenExpired
	}

	return refreshToken.UserID, nil
}

func (s *RefreshTokenStorage) RefreshDelete(token string) error {
	return s.queries.DeleteRefreshToken(context.Background(), token)
}

func (s *RefreshTokenStorage) RefreshDeleteByUserID(userID int64) error {
	return s.queries.RefreshDeleteByUserID(context.Background(), userID)
}

internal/auth/adapters/twofaHandler.go:
package adapters

import (
	"krampus/internal/user/domain"
	"krampus/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TwoFAService interface {
	VerifyCode(code domain.Code) (domain.TokenResponse, error)
	EnableTwoFA(userID int64) error
	// DisableTwoFA(userID int64, passqord string) error
}

type twoFAHandler struct {
	twoFAService TwoFAService
	logger       *logging.Logger
}

func NewTwoFAHandler(s TwoFAService, l *logging.Logger) *twoFAHandler {
	return &twoFAHandler{
		twoFAService: s,
		logger:       l,
	}
}

func (h *twoFAHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/verify", h.VerifyCode)
	rg.POST("/enable", h.EnableTwoFA)
	// rg.POST("/disable", h.DisableTwoFA) // раскомментируйте, когда метод будет готов
}

func (h *twoFAHandler) VerifyCode(c *gin.Context) {
	var code domain.Code
	if err := c.ShouldBindJSON(&code); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	tokenRes, err := h.twoFAService.VerifyCode(code)
	if err != nil {
		h.logger.Error("Failed to verify code: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Verification failed",
		})
		return
	}

	c.JSON(http.StatusOK, tokenRes)
}

func (h *twoFAHandler) EnableTwoFA(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	err := h.twoFAService.EnableTwoFA(userID.(int64))
	if err != nil {
		h.logger.Error("Failed to enable 2FA: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to enable 2FA",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// func (h *twoFAHandler) DisableTwoFA(c *gin.Context) {
// 	userID, exists := c.Get("userID")
// 	if !exists {
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Unauthorized",
// 		})
// 		return
// 	}

// 	var req domain.TwoFaToggleRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		h.logger.Error("Failed to bind JSON: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Invalid request body",
// 		})
// 		return
// 	}

// 	err := h.twoFAService.DisableTwoFA(userID.(int64), req.Password)
// 	if err != nil {
// 		h.logger.Error("Failed to enable 2FA: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Failed to enable 2FA",
// 		})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"success": true,
// 	})
// }

internal/auth/domain/twofa.go:
package domain

import "time"

type TwoFaCodes struct {
	RequiresTwoFa bool   `json:"requires_two_fa"`
	TempToken     string `json:"temp_token"`
}

type TwoFaCode struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
	IsUsed    bool      `json:"is_used"`
	CreatedAt time.Time `json:"created_at"`
}

internal/auth/service/twofaService.go:
package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"krampus/internal/auth/domain"
	user "krampus/internal/user/domain"
	userService "krampus/internal/user/service"
	userStorage "krampus/internal/user/storage"

	"github.com/golang-jwt/jwt/v5"
)

type TwoFAStorage interface {
	RenovationTwoFAStatus(userID int64, enabled bool) error
	InsertTwoFaCode(userID int64, code string, expiresAt time.Time) error
	SelectTwoFaCodeByUserID(userID int64) (domain.TwoFaCode, error)
	RenovationTwoFaCodeAttempts(codeID int64, attempts int64) error
	MarkTwoFaCodeUsed(codeID int64) error
	SelectRecentCodeRequests(userID int64, since time.Time) (int64, error)
	SelectRecentVerificationAttempts(userID int64, since time.Time) (int64, error)
}

type TwoFA struct {
	twoFAStorage        TwoFAStorage
	refreshTokenService *userService.RefreshToken
	redisStorage        *userStorage.RedisSessionStorage
	jwtSecret           string
}

func NewTwoFA(twoFA TwoFAStorage, refreshToken *userService.RefreshToken, redisStorage *userStorage.RedisSessionStorage, jwt string) *TwoFA {
	return &TwoFA{
		twoFAStorage:        twoFA,
		refreshTokenService: refreshToken,
		redisStorage:        redisStorage,
		jwtSecret:           jwt,
	}
}

func (s *TwoFA) EnableTwoFA(userID int64) error {
	return s.twoFAStorage.RenovationTwoFAStatus(userID, true)
}

func (s *TwoFA) UsersSendEmailCode(tempToken string) error {
	var userID int64

	cachedTemp, err := s.redisStorage.GetTempToken(tempToken)
	if err == nil && time.Now().Before(cachedTemp.ExpiresAt) {
		userID = cachedTemp.UserID
	} else {
		userID, err = s.extractUserIDFromToken(tempToken)
		if err != nil {
			return errors.New("Invalid temp token")
		}

		tempTokenData := user.CachedTempToken{
			UserID:    userID,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		s.redisStorage.SetTempToken(tempToken, tempTokenData)
	}

	fifteenMinutesAgo := time.Now().Add(-15 * time.Minute)
	recentRequests, err := s.twoFAStorage.SelectRecentCodeRequests(userID, fifteenMinutesAgo)
	if err != nil {
		return err
	}

	if recentRequests >= 3 {
		return errors.New("too many code requests, please try again later")
	}

	code, err := s.generateSixDigitCode()
	if err != nil {
		return errors.New("failed to generate code")
	}

	expiresAt := time.Now().Add(5 * time.Minute)
	err = s.twoFAStorage.InsertTwoFaCode(userID, code, expiresAt)
	if err != nil {
		return err
	}

	err = s.sendEmail(userID, code)
	if err != nil {
		return err
	}

	return nil
}

func (s *TwoFA) VerifyCode(code user.Code) (user.TokenResponse, error) {
	userID, err := s.extractUserIDFromToken(code.TempToken)
	if err != nil {
		return user.TokenResponse{}, errors.New("invalid temp token")
	}

	tenMinuteAgo := time.Now().Add(-10 * time.Minute)
	recentAttempts, err := s.twoFAStorage.SelectRecentVerificationAttempts(userID, tenMinuteAgo)
	if err != nil {
		return user.TokenResponse{}, err
	}

	if recentAttempts >= 5 {
		return user.TokenResponse{}, errors.New("too many verification attempts, please try again later")
	}

	twoFaCode, err := s.twoFAStorage.SelectTwoFaCodeByUserID(userID)
	if err != nil {
		return user.TokenResponse{}, errors.New("invalid temp token or code not found")
	}

	if twoFaCode.IsUsed {
		return user.TokenResponse{}, errors.New("code already used")
	}

	if twoFaCode.Attempts >= 3 {
		return user.TokenResponse{}, errors.New("too many attempts")
	}

	if time.Now().After(twoFaCode.ExpiresAt) {
		return user.TokenResponse{}, errors.New("code expires")
	}

	if twoFaCode.Code != code.Code {
		err = s.twoFAStorage.RenovationTwoFaCodeAttempts(twoFaCode.ID, int64(twoFaCode.Attempts+1))
		if err != nil {
			return user.TokenResponse{}, err
		}

		remainingAttempts := 3 - (twoFaCode.Attempts + 1)
		return user.TokenResponse{}, fmt.Errorf("invalid code, %d attempts remaining", remainingAttempts)
	}

	err = s.twoFAStorage.MarkTwoFaCodeUsed(twoFaCode.ID)
	if err != nil {
		return user.TokenResponse{}, err
	}

	accessToken, err := s.GenerateAccessToken(twoFaCode.UserID)
	if err != nil {
		return user.TokenResponse{}, err
	}

	refreshToken, err := s.refreshTokenService.GenerateRefreshToken(twoFaCode.UserID)
	if err != nil {
		return user.TokenResponse{}, err
	}

	return user.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *TwoFA) generateSixDigitCode() (string, error) {
	max := big.NewInt(899999)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", nil
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func (s *TwoFA) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *TwoFA) extractUserIDFromToken(tokenString string) (int64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("Invalid token claims")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("Invalid user_id in token")
	}

	return int64(userIDFloat), nil
}

func (s *TwoFA) sendEmail(userID int64, code string) error {
	fmt.Printf("Sending email to user %d: Your code: %s (valid for 5 minutes)\n", userID, code)
	return nil
}

internal/auth/storage/psqlTwofaStorage.go:
package storage

import (
	"context"
	"time"

	"krampus/internal/auth/domain"
	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type StorageError struct {
	Message string
}

type Storage struct {
	queries *database.Queries
}

var (
	ErrTokenExpired = &StorageError{"token expired"}
)

func NewStorage(queries *database.Queries) *Storage {
	return &Storage{queries: queries}
}

func (e *StorageError) Error() string {
	return e.Message
}

func (s *Storage) RenovationTwoFAStatus(userID int64, enabled bool) error {
	return s.queries.UpdateTwoFAStatus(context.Background(), database.UpdateTwoFAStatusParams{
		ID:           userID,
		TwoFaEnabled: pgtype.Bool{Bool: enabled, Valid: true},
	})
}

func (s *Storage) InsertTwoFaCode(userID int64, code string, expiresAt time.Time) error {
	_, err := s.queries.CreateTwoFaCode(context.Background(), database.CreateTwoFaCodeParams{
		UserID:    userID,
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	return err
}

func (s *Storage) SelectTwoFaCodeByUserID(userID int64) (domain.TwoFaCode, error) {
	code, err := s.queries.GetTwoFaCodeByUserID(context.Background(), userID)
	if err != nil {
		return domain.TwoFaCode{}, err
	}

	return domain.TwoFaCode{
		ID:        code.ID,
		UserID:    code.UserID,
		Code:      code.Code,
		ExpiresAt: code.ExpiresAt.Time,
		Attempts:  int(code.Attempts.Int32),
		IsUsed:    code.IsUsed.Bool,
		CreatedAt: code.CreatedAt.Time,
	}, nil
}

func (s *Storage) RenovationTwoFaCodeAttempts(codeID int64, attempts int64) error {
	return s.queries.UpdateTwoFaCodeAttempts(context.Background(), database.UpdateTwoFaCodeAttemptsParams{
		ID:       codeID,
		Attempts: pgtype.Int4{Int32: int32(attempts), Valid: true},
	})
}

func (s *Storage) MarkTwoFaCodeUsed(codeID int64) error {
	return s.queries.MarkTwoFaCodeAsUsed(context.Background(), codeID)
}

func (s *Storage) SelectRecentCodeRequests(userID int64, since time.Time) (int64, error) {
	count, err := s.queries.GetRecentCodeRequests(context.Background(), database.GetRecentCodeRequestsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *Storage) SelectRecentVerificationAttempts(userID int64, since time.Time) (int64, error) {
	count, err := s.queries.GetRecentVerificationAttempts(context.Background(), database.GetRecentVerificationAttemptsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}


internal/chat/adapters/chatHandler.go:
package adapters

import (
	"context"
	"krampus/internal/chat/domain"
	messageDomain "krampus/internal/message/domain"
	messageService "krampus/internal/message/service"
	userDomain "krampus/internal/user/domain"
	redisStorage "krampus/internal/user/storage"
	"krampus/pkg/apperror"
	"krampus/pkg/config"
	"krampus/pkg/types"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Room interface {
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	IsRoomMember(ctx context.Context, roomID, userID string) (bool, error)
	CanSendMessage(ctx context.Context, room *domain.Room, user *userDomain.User, msgType messageDomain.MessageType) bool
	CreateRoom(ctx context.Context, room *domain.Room) error
	UpdateRoom(ctx context.Context, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
	ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error)
}

type UserClient interface {
	GetUser(ctx context.Context, id string) (*userDomain.User, error)
	UpdateLastActivity(ctx context.Context, userID string)
	ValidateUserPermissions(ctx context.Context, userID string, required []string) error
	GetUserStatus(userID string) userDomain.ChatUserStatus
	SaveUser(ctx context.Context, user *userDomain.User) error
	UpdateUser(ctx context.Context, user *userDomain.User) error
}

type Router struct {
	RoomService       Room
	UserClientService UserClient
	MessageService    *messageService.MessageService
	config            config.Config
}

func NewRouter(rs *redisStorage.RedisSessionStorage, roomSvc Room, userSvc UserClient, msgSvc *messageService.MessageService, cfg *config.Config) *Router {
	return &Router{
		RoomService:       roomSvc,
		UserClientService: userSvc,
		MessageService:    msgSvc,
		config:            *cfg,
	}
}

func (r *Router) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/messages", handleSendMessage(r.MessageService))
	rg.POST("/rooms", handleCreateRoom(r.RoomService))
	rg.GET("/rooms/:room_id/messages", handleGetMessages(r.MessageService))
	rg.GET("/rooms/:room_id", handleGetRoom(r.RoomService))
	rg.GET("/users/:user_id", handleGetUser(r.UserClientService))
}

func handleSendMessage(s *messageService.MessageService) gin.HandlerFunc {
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
		msg.UserID = types.UserID(
			userID.(string),
		)

		if err := s.Process(c.Request.Context(), &msg); err != nil {
			c.Error(err.(*apperror.AppError)) // уже AppError
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "sent", "msg_id": msg.ID})
	}
}

func handleGetMessages(s *messageService.MessageService) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("room_id")
		limitStr := c.DefaultQuery("limit", "50")
		limit, _ := strconv.Atoi(limitStr)
		if limit < 1 || limit > 1000 {
			limit = 50
		}

		msgs, err := s.GetRoomMessages(c.Request.Context(), roomID, limit)
		if err != nil {
			c.Error(err.(*apperror.AppError))
			return
		}
		c.JSON(http.StatusOK, gin.H{"messages": msgs})
	}
}

func handleCreateRoom(s Room) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id") // auth checked middleware

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

		if err := s.CreateRoom(c.Request.Context(), &room); err != nil {
			c.Error(err.(*apperror.AppError))
			return
		}
		c.JSON(http.StatusOK, gin.H{"room": room})
	}
}

func handleGetRoom(s Room) gin.HandlerFunc {
	return func(c *gin.Context) {
		room, err := s.GetRoom(c.Request.Context(), c.Param("room_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusOK, room)
	}
}

func handleGetUser(s UserClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := s.GetUser(c.Request.Context(), c.Param("user_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

func handleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleMetrics() gin.HandlerFunc {
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

internal/chat/domain/chat.go:
package domain

type RoomType string

const (
	RoomPersonal  RoomType = "personal"
	RoomPrivate   RoomType = "private"
	RoomGroup     RoomType = "group"
	RoomVideoCall RoomType = "video_call"
)

type Room struct {
	ID        string
	Type      RoomType
	OwnerID   string
	Name      string
	Members   []string
	CreatedAt int64
	UpdatedAt int64
	Settings  RoomSettings
}

type RoomMember struct {
	UserID     string
	RoomID     string
	Role       string
	JoinedAt   int64
	Permission []string
}

type RoomSettings struct {
	ReadOnly     bool
	ModerateMsgs bool
	AllowFiles   bool
	MaxMembers   int
	AutoArchive  bool
}

type UserConnection struct {
	UserID      string `json:"user_id"`
	ConnID      string `json:"conn_id"`
	ClientInfo  string `json:"client_info"`
	IP          string `json:"ip"`
	ConnectedAt int64  `json:"connected_at"`
	Transport   string `json:"transport"`
}

internal/chat/service/roomService.go:
package service

import (
	"context"
	"fmt"
	"krampus/internal/chat/domain"
	messageDomain "krampus/internal/message/domain"
	userDomain "krampus/internal/user/domain"
	"krampus/pkg/types"
	"time"
)

type RoomStorage interface {
	SaveRoom(ctx context.Context, room *domain.Room) error
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	UpdateRoom(ctx context.Context, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
	ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error)
}

type RoomCache interface {
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	SetRoom(ctx context.Context, id string, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
}

type RoomService struct {
	storage       RoomStorage
	cache         RoomCache
	userClientSvc *UserClientService
}

func NewRoomService(s RoomStorage, cache RoomCache, userClientSvc *UserClientService) *RoomService {
	return &RoomService{
		storage:       s,
		cache:         cache,
		userClientSvc: userClientSvc,
	}
}

func (rs *RoomService) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	room, err := rs.cache.GetRoom(ctx, id)
	if err == nil && room != nil {
		return room, nil
	}

	room, err = rs.storage.GetRoom(ctx, id)
	if err != nil {
		return nil, err
	}

	go rs.cache.SetRoom(ctx, id, room)
	return room, nil
}

func (rs *RoomService) IsRoomMember(ctx context.Context, roomID, userID string) (bool, error) {
	room, err := rs.GetRoom(ctx, roomID)
	if err != nil {
		return false, err
	}

	return rs.isRoomMember(room, userID), nil
}

func (rs *RoomService) CanSendMessage(ctx context.Context, room *domain.Room, user *userDomain.User, msgType messageDomain.MessageType) bool {
	userID := types.UserIDFromInt64(user.ID).String()
	if !rs.isRoomMember(room, userID) {
		return msgType == messageDomain.TypeSystem
	}

	switch room.Type {
	case domain.RoomPersonal:
		return room.OwnerID == userID
	case domain.RoomPrivate:
		return true
	case domain.RoomGroup:
		return true
	case domain.RoomVideoCall:
		return rs.isCallActive(room)
	}

	if room.Settings.ReadOnly && msgType != messageDomain.TypeSystem {
		return false
	}
	if !room.Settings.AllowFiles && msgType == messageDomain.TypeFile {
		return false
	}
	return true
}

func (rs *RoomService) isRoomMember(room *domain.Room, userID string) bool {
	for _, memberID := range room.Members {
		if memberID == userID {
			return true
		}
	}
	return false
}

// Заглушка, написать логику проверки статуса звонка
func (rs *RoomService) isCallActive(room *domain.Room) bool {
	if room.Type != domain.RoomVideoCall {
		return false
	}

	return len(room.Members) >= 2
}

func (rs *RoomService) CreateRoom(ctx context.Context, room *domain.Room) error {
	if err := rs.validateRoom(room); err != nil {
		return err
	}

	now := time.Now().UnixNano()
	room.CreatedAt = now
	room.UpdatedAt = now

	if err := rs.storage.SaveRoom(ctx, room); err != nil {
		return err
	}

	go rs.cache.SetRoom(ctx, room.ID, room)
	return nil
}

func (rs *RoomService) validateRoom(room *domain.Room) error {
	if room.ID == "" {
		return fmt.Errorf("room ID cannot be empty")
	}
	if room.OwnerID == "" {
		return fmt.Errorf("room must have an owner")
	}

	switch room.Type {
	case domain.RoomPersonal, domain.RoomPrivate, domain.RoomGroup, domain.RoomVideoCall:
	default:
		return fmt.Errorf("invalid room type: %s", room.Type)
	}

	if room.Type == domain.RoomGroup && len(room.Members) == 0 {
		room.Members = append(room.Members, room.OwnerID)
	}

	return nil
}

func (rs *RoomService) UpdateRoom(ctx context.Context, room *domain.Room) error {
	room.UpdatedAt = time.Now().UnixNano()
	if err := rs.storage.UpdateRoom(ctx, room); err != nil {
		return err
	}
	return rs.cache.SetRoom(ctx, room.ID, room)
}

func (rs *RoomService) DeleteRoom(ctx context.Context, id string) error {
	if err := rs.storage.DeleteRoom(ctx, id); err != nil {
		return err
	}
	return rs.cache.DeleteRoom(ctx, id)
}

func (rs *RoomService) ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	return rs.storage.ListUserRooms(ctx, userID)
}

internal/chat/service/userClientService.go:
package service

import (
	"context"
	"fmt"
	userDomain "krampus/internal/user/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/types"
	"log"
	"time"
)

type UserClientStorage interface {
	SaveUserClient(ctx context.Context, user *userDomain.User) error
	GetUserClient(ctx context.Context, id string) (*userDomain.User, error)
	UpdateUserClient(ctx context.Context, user *userDomain.User) error
	UpdateLastActivity(ctx context.Context, userID string, ts int64) error
}

type UserClientCache interface {
	GetUserClient(ctx context.Context, id string) (*userDomain.User, error)
	SetUserClient(ctx context.Context, id string, user *userDomain.User) error
	DeleteUserClient(ctx context.Context, id string) error
}

type UserClientService struct {
	storage UserClientStorage
	cache   UserClientCache
}

func NewUserClientService(s UserClientStorage, c UserClientCache) *UserClientService {
	return &UserClientService{
		storage: s,
		cache:   c,
	}
}

func (ucs *UserClientService) GetUser(ctx context.Context, id string) (*userDomain.User, error) {
	if user, err := ucs.cache.GetUserClient(ctx, id); err == nil && user != nil {
		return user, nil
	}
	user, err := ucs.storage.GetUserClient(ctx, id)
	if err != nil {
		return nil, apperror.New(apperror.ErrUserNotFound, "user not found")
	}
	go ucs.cache.SetUserClient(ctx, id, user)
	return user, nil
}

func (ucs *UserClientService) UpdateLastActivity(ctx context.Context, userID string) {
	now := time.Now().UnixNano()
	if err := ucs.storage.UpdateLastActivity(ctx, userID, now); err != nil {
		log.Printf("Failed to update activity for %s: %v", userID, err)
		return
	}

	if user, err := ucs.cache.GetUserClient(ctx, userID); err == nil {
		user.LastActive = now
		go ucs.cache.SetUserClient(ctx, userID, user)
	}
}

func (ucs *UserClientService) ValidateUserPermissions(ctx context.Context, userID string, required []string) error {
	user, err := ucs.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	for _, requiredPerm := range required {
		hasPerm := false
		for _, userPerm := range user.Permissions {
			if userPerm == requiredPerm {
				hasPerm = true
				break
			}
		}
		if !hasPerm {
			return apperror.New(apperror.ErrConnection, fmt.Sprintf("missing permission: %s", requiredPerm))
		}
	}
	return nil
}

func (ucs *UserClientService) GetUserStatus(userID string) userDomain.ChatUserStatus {
	user, err := ucs.GetUser(context.Background(), userID)
	if err != nil {
		return userDomain.StatusOffline
	}

	inactiveDuration := time.Since(time.Unix(0, user.LastActive))

	switch {
	case inactiveDuration < 5*time.Minute:
		return userDomain.StatusOnline
	case inactiveDuration < 30*time.Minute:
		return userDomain.StatusAway
	default:
		return userDomain.StatusOffline
	}
}

func (ucs *UserClientService) SaveUser(ctx context.Context, user *userDomain.User) error {
	if err := ucs.storage.SaveUserClient(ctx, user); err != nil {
		return err
	}
	idStr := types.UserIDFromInt64(user.ID).String()
	return ucs.cache.SetUserClient(ctx, idStr, user)
}

func (ucs *UserClientService) UpdateUser(ctx context.Context, user *userDomain.User) error {
	if err := ucs.storage.UpdateUserClient(ctx, user); err != nil {
		return err
	}
	idStr := types.UserIDFromInt64(user.ID).String()
	return ucs.cache.DeleteUserClient(ctx, idStr)
}

internal/chat/storage/roomCache.go:
// internal/chat/storage/roomCache.go - Заполнен пустой
package storage

import (
	"context"
	"encoding/json"
	"time"

	"krampus/internal/chat/domain"
	myredis "krampus/pkg/client-database/redis"

	"github.com/redis/go-redis/v9"
)

type RoomCache struct {
	client *myredis.Client
}

func NewRoomCache(client *myredis.Client) *RoomCache {
	return &RoomCache{client: client}
}

func (c *RoomCache) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	data, err := c.client.RDB().Get(ctx, "room:"+id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var room domain.Room
	if err := json.Unmarshal([]byte(data), &room); err != nil {
		return nil, err
	}
	return &room, nil
}

func (c *RoomCache) SetRoom(ctx context.Context, id string, room *domain.Room) error {
	data, err := json.Marshal(room)
	if err != nil {
		return err
	}
	return c.client.RDB().Set(ctx, "room:"+id, data, 10*time.Minute).Err()
}

func (c *RoomCache) DeleteRoom(ctx context.Context, id string) error {
	return c.client.RDB().Del(ctx, "room:"+id).Err()
}

internal/chat/storage/roomStorage.go:
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"krampus/internal/chat/domain"
	database "krampus/internal/sqlc"
)

type RoomPGStorage struct {
	queries *database.Queries
}

func NewRoomPGStorage(queries *database.Queries) *RoomPGStorage {
	return &RoomPGStorage{queries: queries}
}

func (s *RoomPGStorage) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	row, err := s.queries.GetRoomByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	var members []string
	if err := json.Unmarshal(row.Members, &members); err != nil {
		return nil, fmt.Errorf("failed to unmarshal members: %w", err)
	}

	var settings domain.RoomSettings
	if err := json.Unmarshal(row.Settings, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &domain.Room{
		ID:        row.ID,
		Type:      domain.RoomType(row.Type),
		OwnerID:   row.OwnerID,
		Name:      row.Name,
		Members:   members,
		Settings:  settings,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *RoomPGStorage) SaveRoom(ctx context.Context, room *domain.Room) error {
	membersJSON, err := json.Marshal(room.Members)
	if err != nil {
		return err
	}
	settingsJSON, err := json.Marshal(room.Settings)
	if err != nil {
		return err
	}

	return s.queries.UpsertRoom(ctx, database.UpsertRoomParams{
		ID:        room.ID,
		Type:      string(room.Type),
		OwnerID:   room.OwnerID,
		Name:      room.Name,
		Members:   membersJSON,
		Settings:  settingsJSON,
		CreatedAt: room.CreatedAt,
		UpdatedAt: room.UpdatedAt,
	})
}

func (s *RoomPGStorage) UpdateRoom(ctx context.Context, room *domain.Room) error {
	membersJSON, err := json.Marshal(room.Members)
	if err != nil {
		return err
	}
	settingsJSON, err := json.Marshal(room.Settings)
	if err != nil {
		return err
	}

	return s.queries.UpdateRoom(ctx, database.UpdateRoomParams{
		ID:        room.ID,
		Name:      room.Name,
		Members:   membersJSON,
		Settings:  settingsJSON,
		UpdatedAt: time.Now().UnixNano(),
	})
}

func (s *RoomPGStorage) DeleteRoom(ctx context.Context, id string) error {
	return s.queries.DeleteRoom(ctx, id)
}

func (s *RoomPGStorage) ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	arg := fmt.Sprintf("[\"%s\"]", userID)
	rows, err := s.queries.ListUserRooms(ctx, []byte(arg))
	if err != nil {
		return nil, err
	}

	var rooms []*domain.Room
	for _, row := range rows {
		var members []string
		_ = json.Unmarshal(row.Members, &members)
		var settings domain.RoomSettings
		_ = json.Unmarshal(row.Settings, &settings)

		rooms = append(rooms, &domain.Room{
			ID:        row.ID,
			Type:      domain.RoomType(row.Type),
			OwnerID:   row.OwnerID,
			Name:      row.Name,
			Members:   members,
			Settings:  settings,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return rooms, nil
}

internal/chat/storage/UserClientCache.go:
package storage

import (
	"context"
	"encoding/json"
	"time"

	"krampus/internal/user/domain"
	myredis "krampus/pkg/client-database/redis"

	"github.com/redis/go-redis/v9"
)

type UserClientCache struct {
	client *myredis.Client
}

func NewUserClientCache(client *myredis.Client) *UserClientCache {
	return &UserClientCache{client: client}
}

func (c *UserClientCache) GetUserClient(ctx context.Context, id string) (*domain.User, error) {
	data, err := c.client.RDB().Get(ctx, "user_client:"+id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var user domain.User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *UserClientCache) SetUserClient(ctx context.Context, id string, user *domain.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return nil
	}
	return c.client.RDB().Set(ctx, "user_client:"+id, data, 5*time.Minute).Err()
}

func (c *UserClientCache) DeleteUserClient(ctx context.Context, id string) error {
	return c.client.RDB().Del(ctx, "user_client:"+id).Err()
}

internal/chat/storage/UserClientStorage.go:
package storage

import (
	"context"
	"fmt"
	"strconv"
	"time"

	database "krampus/internal/sqlc"
	"krampus/internal/user/domain"

	"github.com/jackc/pgx/v5/pgtype"
)

type UserClientPGStorage struct {
	queries *database.Queries
}

func NewUserClientPGStorage(queries *database.Queries) *UserClientPGStorage {
	return &UserClientPGStorage{queries: queries}
}

func (s *UserClientPGStorage) GetUserClient(ctx context.Context, id string) (*domain.User, error) {
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user id format: %w", err)
	}

	user, err := s.queries.GetUserByID(ctx, idInt)
	if err != nil {
		return nil, err
	}
	return &domain.User{
		ID:         user.ID,
		Username:   user.Username,
		Email:      user.Email,
		LastActive: user.CreatedAt.Time.UnixNano(),
	}, nil
}

func (s *UserClientPGStorage) UpdateLastActivity(ctx context.Context, userID string, timestamp int64) error {
	idInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return err
	}

	updateTime := time.Unix(0, timestamp)

	return s.queries.UpdateUserLastActive(ctx, database.UpdateUserLastActiveParams{
		ID:        idInt,
		UpdatedAt: pgtype.Timestamptz{Time: updateTime, Valid: true},
	})
}

func (s *UserClientPGStorage) SaveUserClient(ctx context.Context, user *domain.User) error {
	return s.queries.CreateUserClient(ctx, database.CreateUserClientParams{
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: "system_generated",
	})
}

func (s *UserClientPGStorage) UpdateUserClient(ctx context.Context, user *domain.User) error {
	return s.queries.UpdateUserClient(ctx, database.UpdateUserClientParams{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
}

internal/identity/domain/auth_context.go:
package domain

import "krampus/pkg/types"

type AuthContext struct {
	UserID        types.UserID
	SessionID     types.SessionID
	TraceID       string
	RequestID     string
	CorrelationID string
}

internal/identity/service/jwt_service.go:
package service

import (
	"errors"
	"krampus/pkg/types"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secret string
}

func NewJWTService(secret string) *JWTService {

	return &JWTService{
		secret: secret,
	}
}

type Claims struct {
	UserID    types.UserID    `json:"user_id"`
	SessionID types.SessionID `json:"session_id"`

	jwt.RegisteredClaims
}

func (s *JWTService) Validate(
	tokenString string,
) (*Claims, error) {

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {

			return []byte(s.secret), nil
		},
	)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)

	if !ok {
		return nil, errors.New("невалидные claims")
	}

	if claims.ExpiresAt == nil {
		return nil, errors.New("отсутствует expiration")
	}

	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, errors.New("токен истёк")
	}

	return claims, nil
}

internal/identity/service/ws_auth_service.go:
package service

import (
	"context"
	"errors"
	"net/http"

	identityDomain "krampus/internal/identity/domain"
	"krampus/pkg/ctxmeta"
	"krampus/pkg/types"
)

type SessionValidator interface {
	ValidateSession(
		ctx context.Context,
		sessionID types.SessionID,
		userID types.UserID,
	) error
}

type RoomAccessValidator interface {
	IsRoomMember(
		ctx context.Context,
		roomID types.RoomID,
		userID types.UserID,
	) (bool, error)
}

type WSAuthService struct {
	jwtService *JWTService
	sessions   SessionValidator
	rooms      RoomAccessValidator
}

func NewWSAuthService(
	jwtService *JWTService,
	sessions SessionValidator,
	rooms RoomAccessValidator,
) *WSAuthService {

	return &WSAuthService{
		jwtService: jwtService,
		sessions:   sessions,
		rooms:      rooms,
	}
}

func (s *WSAuthService) Authenticate(
	ctx context.Context,
	r *http.Request,
) (*identityDomain.AuthContext, error) {

	token := r.URL.Query().Get("token")

	if token == "" {
		return nil, errors.New("token missing")
	}

	roomID := types.RoomID(
		r.URL.Query().Get("room_id"),
	)

	if roomID == "" {
		return nil, errors.New("room_id missing")
	}

	claims, err := s.jwtService.Validate(token)

	if err != nil {
		return nil, err
	}

	err = s.sessions.ValidateSession(
		ctx,
		claims.SessionID,
		claims.UserID,
	)

	if err != nil {
		return nil, err
	}

	isMember, err := s.rooms.IsRoomMember(
		ctx,
		roomID,
		claims.UserID,
	)

	if err != nil {
		return nil, err
	}

	if !isMember {
		return nil, errors.New("access denied")
	}

	meta := ctxmeta.Extract(ctx)

	return &identityDomain.AuthContext{
		UserID:        claims.UserID,
		SessionID:     claims.SessionID,
		TraceID:       meta.TraceID,
		RequestID:     meta.RequestID,
		CorrelationID: meta.CorrelationID,
	}, nil
}

internal/platform/supervisor/supervisor.go:
package supervisor

import (
	"context"
	"log"
	"runtime/debug"
	"time"
)

type WorkerFunc func(ctx context.Context)

func RunWorker(ctx context.Context, name string, fn WorkerFunc) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("worker panic name=%s panic=%v stack=%s", name, r, string(debug.Stack()))
						time.Sleep(time.Second)
					}
				}()
				fn(ctx)
			}()
		}
	}()
}
