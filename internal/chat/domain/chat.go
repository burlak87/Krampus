package domain

type RoomType string

const (
	RoomPersonal  RoomType = "personal"
	RoomPrivate   RoomType = "private"
	RoomGroup     RoomType = "group"
	RoomVideoCall RoomType = "video_call"
)

type Room struct {
	ID        string       `json:"id"`
	Type      RoomType     `json:"type"`
	OwnerID   string       `json:"owner_id"`
	Name      string       `json:"name"`
	Members   []string     `json:"members"`
	CreatedAt int64        `json:"created_at"`
	UpdatedAt int64        `json:"updated_at"`
	Settings  RoomSettings `json:"settings"`
	Avatar    string       `json:"avatar"`
}

type RoomMember struct {
	UserID     string
	RoomID     string
	Role       string
	JoinedAt   int64
	Permission []string
}

type RoomSettings struct {
	ReadOnly     bool `json:"read_only"`
	ModerateMsgs bool `json:"moderate_msgs"`
	AllowFiles   bool `json:"allow_files"`
	MaxMembers   int  `json:"max_members"`
	AutoArchive  bool `json:"auto_archive"`
}

type UserConnection struct {
	UserID      string `json:"user_id"`
	ConnID      string `json:"conn_id"`
	ClientInfo  string `json:"client_info"`
	IP          string `json:"ip"`
	ConnectedAt int64  `json:"connected_at"`
	Transport   string `json:"transport"`
}
