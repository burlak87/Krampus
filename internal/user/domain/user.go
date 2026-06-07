package domain

import "time"

type ChatUserStatus string

const (
	StatusOnline  ChatUserStatus = "online"
	StatusAway    ChatUserStatus = "away"
	StatusOffline ChatUserStatus = "offline"
	StatusDND     ChatUserStatus = "dnd"
)

type User struct {
	ID           int64                  `json:"id"`
	Username     string                 `json:"username"`
	Firstname    string                 `json:"firstname"`
	Lastname     string                 `json:"lastname"`
	Email        string                 `json:"email"`
	PasswordHash string                 `json:"-"`
	Avatar       string                 `json:"avatar"`
	TwoFAEnabled bool                   `json:"two_fa_enabled"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActive   int64                  `json:"last_active"`
	Status       ChatUserStatus         `json:"status"`
	Permissions  []string               `json:"permissions"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type Code struct {
	TempToken string `json:"temp_token"`
	Code      string `json:"code"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username     string `json:"username"`
	Firstname    string `json:"firstname"`
	Lastname     string `json:"lastname"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	TwoFAEnabled bool   `json:"two_fa_enabled"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type TwoFaToggleRequest struct {
	Password string `json:"password"`
}
