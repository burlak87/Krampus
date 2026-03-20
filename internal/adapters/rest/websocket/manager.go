package websocket

import (
	"krampus/internal/domain"
	"log"
	"sync"
)

type ConnectionManager struct {
	users     sync.Map
	rooms     sync.Map
	userLocks sync.Map
}

type UserConnections struct {
	mu     sync.RWMutex
	conns  map[string]*Client
	userID string
}

type RoomSubscribers struct {
	mu     sync.RWMutex
	users  map[string]bool
	roomID string
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{}
}

func (m *ConnectionManager) Register(client *Client) error {
	userID, roomID := client.UserID, client.RoomID

	ucI, _ := m.users.LoadOrStore(userID, &UserConnections{
		conns:  make(map[string]*Client),
		userID: userID,
	})
	uc := ucI.(*ConnectionManager)

	lockI, _ := m.userLocks.LoadOrStore(userID, new(sync.Mutex))
	userLock := lockI.(*sync.Mutex)
	userLock.Lock()
	defer userLock.Unlock()

	uc.mu.Lock()
	uc.conns[client.ConnID()] = client
	uc.mu.Unlock()

	m.subscribeToRoom(roomID, userID)
	client.Start()

	log.Printf("WS registered: %s@%s (%s)", userID, roomID, client.ConnID())
	return nil
}

func (m *ConnectionManager) Unregister(client *Client) {
	userID, roomID := client.UserID, client.RoomID

	if ucI, ok := m.users.Load(userID); ok {
		uc := ucI.(*ConnectionManager)
		LockI, _ := m.userLocks.Load(userID)
		userLock := LockI.(*sync.Mutex)

		userLock.Lock()
		defer userLock.Unlock()

		uc.mu.Lock()
		delete(uc.conns, client.ConnID())
		uc.mu.Unlock()

		if len(uc.conns) == 0 {
			m.unsubscribeFromRoom(roomID, userID)
			// m.users.Delete(userID)
			// m.userLocks.Delete(userID)
		}
	}
}

func (m *ConnectionManager) BroadcastToRoom(roomID string, msg *domain.BaseMessage) error {
	subsI, ok := m.rooms.Load(roomID)
	if !ok {
		return nil
	}
	subs := subsI.(*RoomSubscribers)
	subs.mu.RLock()
	defer subs.mu.RUnlock()

	var wg sync.WaitGroup
	for userID := range subs.users {
		wg.Add(1)
		go func(targetUserID string) {
			defer wg.Done()
			m.SendToUser(targetUserID, msg)
		}(userID)
	}
	wg.Wait()

	return nil
}

func (m *ConnectionManager) SendToUser(userID string, msg *domain.BaseMessage) error {
	ucI, ok := m.users.Load(userID)
	if !ok {
		return nil
	}
	uc := ucI.(*UserConnections)
	LockI, _ := m.userLocks.LoadOrStore(userID, new(sync.Mutex))
	userLock := LockI.(*sync.Mutex)

	userLock.Lock()
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	defer userLock.Unlock()

	for _, client := range uc.conns {
		select {
		case client.Send <- msg:
		default:
			log.Printf("Dropped msg for slow client: %s", client.ConnID())
		}
	}
	return nil
}

func (m *ConnectionManager) subscribeToRoom(roomID, userID string) {
	subsI, _ := m.rooms.LoadOrStore(roomID, &RoomSubscribers{
		users:  make(map[string]bool),
		roomID: roomID,
	})
	subs := subsI.(*RoomSubscribers)
	subs.mu.Lock()
	subs.users[userID] = true
	subs.mu.Unlock()
}

func (m *ConnectionManager) unsubscribeFromRoom(roomID, userID string) {
	if subsI, ok := m.rooms.Load(roomID); ok {
		subs := subsI.(*RoomSubscribers)
		subs.mu.Lock()
		delete(subs.users, userID)
		subs.mu.Unlock()
	}
}
