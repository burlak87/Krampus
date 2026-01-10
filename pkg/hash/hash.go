package hash

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

func GenerateHash(password string) (string, error) {
	if password == "" || len(password) < 0 {
		return "", errors.New("password must be at least 8 characters")
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("error hashing password")
	}
	
	return string(hash), nil
}

func CompareHashAndPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}