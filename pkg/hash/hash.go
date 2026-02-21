package hash

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

func GenerateHash(password string) (string, error) {
	if password == "" || len(password) == 0 {
		return "", errors.New("password must be at least 8 characters")
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("error hashing password")
	}
	
	return string(hash), nil
}

func GenerateHashPasskey(passkey string) (string, error) {
	if passkey == "" || len(passkey) == 0 {
		return "", errors.New("passkey must be at least 4 characters")
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(passkey), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("error hashing passkey")
	}
	
	return string(hash), nil
}

func GenerateHashWordpasskey(wordpasskey string) (string, error) {
	if wordpasskey == "" || len(wordpasskey) == 0 {
		return "", errors.New("wordpasskey must be at least 6 characters")
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(wordpasskey), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("error hashing wordpasskey")
	}
	
	return string(hash), nil
}

func GenerateHashCloudPassword() {
	
}

func CompareHashAndPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}