package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"strings"
)

const (
	rawKeyPrefix        = "rls_live_"
	displayPrefixLength = 8
)

type APIKeyCodec struct {
	pepper []byte
}

func NewAPIKeyCodec(pepper string) *APIKeyCodec {
	return &APIKeyCodec{
		pepper: []byte(pepper),
	}
}

func (c *APIKeyCodec) Generate() (string, string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", err
	}

	encoded := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret))
	rawKey := rawKeyPrefix + encoded
	keyPrefix := rawKeyPrefix + encoded[:displayPrefixLength]

	return rawKey, keyPrefix, nil
}

func (c *APIKeyCodec) Hash(rawKey string) string {
	mac := hmac.New(sha256.New, c.pepper)
	mac.Write([]byte(rawKey))
	return hex.EncodeToString(mac.Sum(nil))
}
