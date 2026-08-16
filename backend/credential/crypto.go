package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

type localEncryptor struct {
	aead cipher.AEAD
}

func NewLocalEncryptor(key []byte) (Encryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("credential encryption key must be exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create credential cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create credential AEAD: %w", err)
	}
	return &localEncryptor{aead: aead}, nil
}

// FromEnvironment uses OPSK_CREDENTIAL_KEY in base64 or as a 32-byte value.
// Development has a deterministic local key so the compose setup remains simple;
// production must always provide an explicit key.
func FromEnvironment(environment string) (Encryptor, error) {
	value := os.Getenv("OPSK_CREDENTIAL_KEY")
	if value == "" {
		if environment == "production" {
			return nil, fmt.Errorf("OPSK_CREDENTIAL_KEY is required in production")
		}
		digest := sha256.Sum256([]byte("opskeeper-development-credential-key"))
		return NewLocalEncryptor(digest[:])
	}
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		key = []byte(value)
	}
	return NewLocalEncryptor(key)
}

func (e *localEncryptor) Encrypt(plaintext []byte) ([]byte, string, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", fmt.Errorf("generate credential nonce: %w", err)
	}
	return e.aead.Seal(nonce, nonce, plaintext, nil), "local-v1", nil
}
