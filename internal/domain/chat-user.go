package domain

type ChatUserStatus string

const (
	StatusOnline  ChatUserStatus = "online"
	StatusAway    ChatUserStatus = "away"
	StatusOffline ChatUserStatus = "offline"
	StatusDND     ChatUserStatus = "dnd"
)

type ChatUser struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Email       string                 `json:"email"`
	Status      ChatUserStatus         `json:"status"`
	LastActive  int64                  `json:"last_active"`
	CreatedAt   int64                  `json:"created_at"`
	Permissions []string               `json:"permissions"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type UserConnection struct {
	UserID      string `json:"user_id"`
	ConnID      string `json:"conn_id"`
	ClientInfo  string `json:"client_info"`
	IP          string `json:"ip"`
	ConnectedAt int64  `json:"connected_at"`
	Transport   string `json:"transport"`
}
