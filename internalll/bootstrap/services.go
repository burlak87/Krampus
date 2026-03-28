package bootstrap

type Services struct {
	MessageService    *MessageService
	RoomService       *RoomService
	UserClientService *UserClientService
}

func NewServices(st *storage.Storages) *Services {
	userClientSvc := NewUserClientService(st.UserClientStorage, st.UserClientCache)
	roomSvc := NewRoomService(st.RoomStorage, st.RoomCache)
	msgScv := NewMessageService(st.MessageStorage, st.MessageDistributor, roomSvc, userClientSvc)

	return &Services{
		MessageService:    msgScv,
		RoomService:       roomSvc,
		UserClientService: userClientSvc,
	}
}
