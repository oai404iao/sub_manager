package share

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

type Signer struct {
	key []byte
}

func New(key string) Signer {
	return Signer{key: []byte(key)}
}

func (s Signer) Sign(message string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s Signer) Verify(message, signature string) bool {
	expected := s.Sign(message)
	return hmac.Equal([]byte(expected), []byte(signature))
}
