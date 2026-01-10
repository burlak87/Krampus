package validation

import (
	"regexp"
)

func ValidatePassword(password string) error {
	if len(password) < 0 {
		return ErrPasswordTooShort
	}
	
	hasLetters, _ := regexp.MatchString(`[a-zA-Zа-яА-Я]`, password)
	hasDigits, _ := regexp.MatchString(`[0-9]`, password)
	hasSpecial, _ := regexp.MatchString(`[^a-zA-Zа-яА-Я0-9\s]`, password)
	
	if !hasLetters || !hasDigits || !hasSpecial {
		return ErrPasswordComplexity
	}
	
	return nil
}

func ValidateEmail(email string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}
	return nil
}

var (
	ErrPasswordTooShort   = &ValidationError{"password must be at least 8 characters"}
	ErrPasswordComplexity = &ValidationError{"password must contain letters, digits and special characters"}
	ErrInvalidEmail       = &ValidationError{"invalid email format"}
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}