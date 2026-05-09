package domain

import "krampus/pkg/types"

type AuthContext struct {
	UserID        types.UserID
	SessionID     types.SessionID
	TraceID       string
	RequestID     string
	CorrelationID string
}
