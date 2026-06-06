package apperror

import (
	"fmt"
)

type ErrorCode string

const (
	ErrInvalidMessage  ErrorCode = "INVALID_MESSAGE"
	ErrUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrRateLimit       ErrorCode = "RATE_LIMIT"
	ErrRoomNotFound    ErrorCode = "ROOM_NOT_FOUND"
	ErrUserNotFound    ErrorCode = "USER_NOT_FOUND"
	ErrForbidden       ErrorCode = "FORBIDDEN"
	ErrStorage         ErrorCode = "STORAGE_ERROR"
	ErrConnection      ErrorCode = "CONNECTION_ERROR"
	ErrValidation      ErrorCode = "VALIDATION_ERROR"
	ErrDuplicate       ErrorCode = "DUPLICATE_ERROR"
	ErrTimeout         ErrorCode = "TIMEOUT_ERROR"
	ErrPayloadTooLarge ErrorCode = "PAYLOAD_TOO_LARGE"
	ErrInternal        ErrorCode = "INTERNAL_ERROR"
)

type AppError struct {
	Code             ErrorCode `json:"code,omitempty"`
	Message          string    `json:"message,omitempty"`
	DeveloperMessage string    `json:"developer_message,omitempty"`
	Details          string    `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func New(code ErrorCode, msg string) *AppError {
	return &AppError{
		Code:    code,
		Message: msg,
	}
}

func NewWithDetails(code ErrorCode, msg, details string) *AppError {
	return &AppError{
		Code:    code,
		Message: msg,
		Details: details,
	}
}
