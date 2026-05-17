package apperror

func IsRetryable(err error) bool {
	appErr, ok := err.(*AppError)
	if !ok {
		return false
	}

	switch appErr.Code {
	case ErrTimeout, ErrConnection, ErrInternal, ErrStorage:
		return true
	default:
		return false
	}
}
