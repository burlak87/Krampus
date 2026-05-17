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
