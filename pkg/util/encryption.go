package util

import "golang.org/x/crypto/bcrypt"

func Encrypt(data string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(data), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func Compare(hashedData, plainData string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedData), []byte(plainData))
}
