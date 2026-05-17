package adapters

type TransportKind string

const (
	TransportWebSocket TransportKind = "websocket"
	TransportSSE       TransportKind = "sse"
)
