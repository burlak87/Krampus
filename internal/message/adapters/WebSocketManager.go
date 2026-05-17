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

	err = m.deliveryRepo.UpdateStatus(
		ctx,
		job.Message.ID,
		job.UserID,
		message.DeliverySent,
	)

	if err != nil {

		log.Printf(
			"delivery sent persist failed message_id=%s user_id=%s err=%v",
			job.Message.ID,
			job.UserID,
			err,
		)
	}

	return nil
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
