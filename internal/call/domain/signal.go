package domain

// Signal type constants relayed through a call room. These mirror the envelope
// `type` values produced by the Nuxt client (frontend/types/signaling.ts,
// frontend/composabels/*). The signaling server is a dumb relay: it does not
// interpret most of these, it only fans them out to the other peers in a room.
const (
	// TypeCheckCount is the bootstrap frame a call client sends right after
	// connecting to learn how many call-peers are already in the room.
	TypeCheckCount = "checkCountUserCall"

	TypeStartStream     = "StartStream"
	TypeStatus          = "Status"
	TypeAnswer          = "Answer"
	TypeCheckUserActive = "checkUserActive"
	TypeICECandidate    = "iceCandidate"
)

// TypeWS distinguishes a media/call connection from a plain chat connection,
// matching the PoC's `typeWS` path segment.
const (
	TypeWSCall = "call"
)

// Envelope is the minimal shape we decode from inbound frames. Everything else
// is relayed verbatim, so we only peek at `type` to handle the bootstrap.
type Envelope struct {
	Type string `json:"type"`
}

// CheckCountResponse is the reply to a TypeCheckCount bootstrap frame.
type CheckCountResponse struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// ICEServer is a single STUN/TURN entry, shaped for direct use in the browser's
// RTCConfiguration.iceServers.
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// ICEServersResponse is returned by GET /api/v1/chat/call/ice-servers.
type ICEServersResponse struct {
	ICEServers []ICEServer `json:"iceServers"`
}
