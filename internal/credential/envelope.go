// Package credential protects provider credentials for manual secret-store import.
package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

type Data struct {
	Token  string `json:"token"`
	Region string `json:"region"`
	UserID int64  `json:"user_id"`
}

var ErrInvalid = errors.New("credential envelope or encryption key invalid")

const purpose = "pcloud-mcp/provider-credential/v1"

// Keys must be independently generated random secrets, never human passwords.
func ValidKey(key string) bool {
	if len(key) < 32 || len(key) > 128 {
		return false
	}
	for _, c := range key {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}

func valid(data Data) bool {
	return data.Token != "" && len(data.Token) <= 4096 && !strings.ContainsAny(data.Token, "\r\n") && (data.Region == "eu" || data.Region == "us") && data.UserID > 0
}
func aead(key string) (cipher.AEAD, error) {
	if !ValidKey(key) {
		return nil, ErrInvalid
	}
	digest := sha256.Sum256([]byte(purpose + "/encryption-key/" + key))
	block, err := aes.NewCipher(digest[:])
	if err != nil {
		return nil, ErrInvalid
	}
	return cipher.NewGCM(block)
}
func Seal(key string, data Data) (string, error) {
	if !valid(data) {
		return "", ErrInvalid
	}
	gcm, err := aead(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", ErrInvalid
	}
	plain, err := json.Marshal(data)
	if err != nil {
		return "", ErrInvalid
	}
	sealed := gcm.Seal(nonce, nonce, plain, []byte(purpose))
	return "v1." + base64.RawURLEncoding.EncodeToString(sealed), nil
}
func Open(key, envelope string) (Data, error) {
	var data Data
	if len(envelope) > 8192 || !strings.HasPrefix(envelope, "v1.") {
		return data, ErrInvalid
	}
	gcm, err := aead(key)
	if err != nil {
		return data, err
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(envelope, "v1."))
	if err != nil || len(raw) < gcm.NonceSize()+gcm.Overhead() {
		return data, ErrInvalid
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte(purpose))
	if err != nil || json.Unmarshal(plain, &data) != nil || !valid(data) {
		return Data{}, ErrInvalid
	}
	return data, nil
}
