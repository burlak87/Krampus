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
