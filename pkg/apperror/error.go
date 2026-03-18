package apperror

import "encoding/json"

type ErrorCode string

const (
	ErrInvalidMessage ErrorCode = "INVALID_MESSAGE"
	ErrUnauthorized   ErrorCode = "UNAUTHORIZED"
	ErrRateLimit      ErrorCode = "RATE_LIMIT"
	ErrRoomNotFound   ErrorCode = "ROOm_NOT_FOUND"
	ErrUserNotFound   ErrorCode = "USER_NOT_FOUND"
	ErrForbidden      ErrorCode = "FORBIDDEN"
	ErrStorage        ErrorCode = "STORAGE_ERROR"
	ErrConnection     ErrorCode = "CONNECTION_ERROR"
)

var (
	ErrNotFound = NewAppError(nil, "not found", "", "US-000003")
	ErrServer   = NewAppError(nil, "Server", "", "")
)

type AppError struct {
	Code             ErrorCode `json:"code,omitempty"`
	Message          string    `json:"message,omitempty"`
	DeveloperMessage string    `json:"developer_message,omitempty"`
	Details          string    `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) Marshal() []byte {
	marshal, err := json.Marshal(e)
	if err != nil {
		return nil
	}
	return marshal
}
 
func NewAppError(err error, message, developerMessage, code string) *AppError {
	return &AppError{
		Err:              err,
		Message:          message,
		DeveloperMessage: developerMessage,
		Code:             code,
	}
}

func systemError(err error) *AppError {
	return NewAppError(err, "internal system error", err.Error(), "US-000000")
}

// func (e *AppError) Error() string { return string(e.Code) + ": " + e.Message }

// func New(code ErrorCode, msg string) *AppError {
// 	return &AppError{Code: code, Message: msg}
// }

// func NewWithDetails(code ErrorCode, msg, details string) *AppError {
// 	return &AppError{Code: code, Message: msg, Details: details}
// }