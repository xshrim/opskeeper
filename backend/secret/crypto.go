package secret

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

type Encryptor interface {
	Encrypt([]byte) ([]byte, string, error)
}

type Decryptor interface {
	Decrypt([]byte, string) ([]byte, error)
}

type localEncryptor struct{ aead cipher.AEAD }

func NewLocalEncryptor(key []byte) (Encryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secret encryption key must be exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret AEAD: %w", err)
	}
	return &localEncryptor{aead: aead}, nil
}

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
		return nil, "", fmt.Errorf("generate secret nonce: %w", err)
	}
	return e.aead.Seal(nonce, nonce, plaintext, nil), "local-v1", nil
}

func (e *localEncryptor) Decrypt(ciphertext []byte, _ string) ([]byte, error) {
	nonceSize := e.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("secret ciphertext is truncated")
	}
	plaintext, err := e.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	return plaintext, nil
}
