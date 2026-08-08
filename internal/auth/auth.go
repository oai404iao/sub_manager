package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	value, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(value), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func NewToken() (plain, digest string, err error) {
	buffer := make([]byte, 32)
	if _, err = rand.Read(buffer); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(buffer)
	digest = Digest(plain)
	return plain, digest, nil
}

func Digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
