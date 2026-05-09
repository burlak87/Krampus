package adapters

import (
	"context"
	"log"
	"sync"

	message "krampus/internal/message/domain"
	"krampus/pkg/ctxmeta"
	"krampus/pkg/messaging/kafka"
	"krampus/pkg/types"

	"github.com/gorilla/websocket"
)

const roomWorkers = 8

type ConnectionManager struct {
	users sync.Map
	rooms sync.Map

	roomQueues sync.Map

	kafkaConsumer *kafka.Consumer
}

type UserConnections struct {
	mu sync.RWMutex

	conns map[string]*Client
}

type RoomSubscribers struct {
	mu sync.RWMutex

	users map[types.UserID]map[string]*Client
}

func NewConnectionManager(
	kafkaConsumer *kafka.Consumer,
) *ConnectionManager {

	m := &ConnectionManager{
		kafkaConsumer: kafkaConsumer,
	}

	for i := 0; i < roomWorkers; i++ {

		queue := make(
			chan *message.BaseMessage,
			1024,
		)

		m.roomQueues.Store(i, queue)

		go m.roomWorker(queue)
	}

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

			m.BroadcastToRoom(
				ctx,
				msg.RoomID,
				msg,
			)
		},
	)

	go m.kafkaConsumer.Consume(
		context.Background(),
	)

	return m
}

func (m *ConnectionManager) roomWorker(
	queue chan *message.BaseMessage,
) {

	for msg := range queue {

		ctx := ctxmeta.WithMetadata(
			context.Background(),
			ctxmeta.Metadata{
				TraceID:       msg.Metadata.TraceID,
				RequestID:     msg.Metadata.RequestID,
				CorrelationID: msg.Metadata.CorrelationID,
				UserID:        msg.UserID.String(),
			},
		)

		m.broadcast(
			ctx,
			msg,
		)
	}
}

func (m *ConnectionManager) Register(
	client *Client,
) error {

	userID := client.UserID
	roomID := client.RoomID

	ucI, _ := m.users.LoadOrStore(
		userID,
		&UserConnections{
			conns: make(map[string]*Client),
		},
	)

	uc := ucI.(*UserConnections)

	uc.mu.Lock()

	uc.conns[client.ConnID()] = client

	uc.mu.Unlock()

	m.subscribeToRoom(
		roomID,
		userID,
		client,
	)

	client.Start()

	meta := ctxmeta.Extract(client.ctx)

	log.Printf(
		"client registered trace_id=%s user_id=%s room_id=%s",
		meta.TraceID,
		userID,
		roomID,
	)

	return nil
}

func (m *ConnectionManager) Unregister(
	client *Client,
) {

	userID := client.UserID
	roomID := client.RoomID

	if ucI, ok := m.users.Load(userID); ok {

		uc := ucI.(*UserConnections)

		uc.mu.Lock()

		delete(
			uc.conns,
			client.ConnID(),
		)

		isEmpty := len(uc.conns) == 0

		uc.mu.Unlock()

		if isEmpty {

			m.users.Delete(userID)
		}
	}

	m.unsubscribeFromRoom(
		roomID,
		userID,
		client,
	)
}

func (m *ConnectionManager) BroadcastToRoom(
	ctx context.Context,
	roomID types.RoomID,
	msg *message.BaseMessage,
) error {

	workerID := hashRoom(
		roomID.String(),
	) % roomWorkers

	queueI, _ := m.roomQueues.Load(workerID)

	queue := queueI.(chan *message.BaseMessage)

	select {

	case queue <- msg:

	default:

		meta := ctxmeta.Extract(ctx)

		log.Printf(
			"room queue overflow trace_id=%s room_id=%s",
			meta.TraceID,
			roomID,
		)
	}

	return nil
}

func (m *ConnectionManager) broadcast(
	ctx context.Context,
	msg *message.BaseMessage,
) {

	subsI, ok := m.rooms.Load(
		msg.RoomID,
	)

	if !ok {
		return
	}

	subs := subsI.(*RoomSubscribers)

	subs.mu.RLock()

	clients := make(
		[]*Client,
		0,
	)

	for _, userClients := range subs.users {

		for _, client := range userClients {

			clients = append(
				clients,
				client,
			)
		}
	}

	subs.mu.RUnlock()

	meta := ctxmeta.Extract(ctx)

	for _, client := range clients {

		select {

		case client.Send <- msg:

		default:

			log.Printf(
				"slow client evicted trace_id=%s user_id=%s",
				meta.TraceID,
				client.UserID,
			)

			client.Close(
				websocket.ClosePolicyViolation,
				"slow consumer",
			)
		}
	}
}

func (m *ConnectionManager) subscribeToRoom(
	roomID types.RoomID,
	userID types.UserID,
	client *Client,
) {

	subsI, _ := m.rooms.LoadOrStore(
		roomID,
		&RoomSubscribers{
			users: make(
				map[types.UserID]map[string]*Client,
			),
		},
	)

	subs := subsI.(*RoomSubscribers)

	subs.mu.Lock()

	if _, ok := subs.users[userID]; !ok {

		subs.users[userID] = make(
			map[string]*Client,
		)
	}

	subs.users[userID][client.ConnID()] = client

	subs.mu.Unlock()
}

func (m *ConnectionManager) unsubscribeFromRoom(
	roomID types.RoomID,
	userID types.UserID,
	client *Client,
) {

	subsI, ok := m.rooms.Load(roomID)

	if !ok {
		return
	}

	subs := subsI.(*RoomSubscribers)

	subs.mu.Lock()

	if userClients, ok := subs.users[userID]; ok {

		delete(
			userClients,
			client.ConnID(),
		)

		if len(userClients) == 0 {

			delete(
				subs.users,
				userID,
			)
		}
	}

	empty := len(subs.users) == 0

	subs.mu.Unlock()

	if empty {

		m.rooms.Delete(roomID)
	}
}

func hashRoom(
	roomID string,
) int {

	hash := 0

	for _, ch := range roomID {

		hash = int(ch) + ((hash << 5) - hash)
	}

	if hash < 0 {
		hash = -hash
	}

	return hash
}
