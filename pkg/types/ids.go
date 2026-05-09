package types

import (
	"fmt"
	"strconv"
)

type UserID string
type RoomID string
type MessageID string
type SessionID string

func UserIDFromInt64(v int64) UserID {
	return UserID(strconv.FormatInt(v, 10))
}

func UserIDFromString(v string) UserID {
	return UserID(v)
}

func RoomIDFromString(v string) RoomID {
	return RoomID(v)
}

func MessageIDFromString(v string) MessageID {
	return MessageID(v)
}

func SessionIDFromString(v string) SessionID {
	return SessionID(v)
}

func (u UserID) String() string {
	return string(u)
}

func (r RoomID) String() string {
	return string(r)
}

func (m MessageID) String() string {
	return string(m)
}

func (s SessionID) String() string {
	return string(s)
}

func (u UserID) Int64() (int64, error) {
	return strconv.ParseInt(string(u), 10, 64)
}

func ParseUserID(v any) (UserID, error) {
	switch t := v.(type) {
	case string:
		return UserID(t), nil
	case int64:
		return UserIDFromInt64(t), nil
	case int:
		return UserID(strconv.Itoa(t)), nil
	default:
		return "", fmt.Errorf("unsupported user id type")
	}
}
