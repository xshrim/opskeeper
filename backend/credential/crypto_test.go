package credential

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestLocalEncryptorEncryptsWithRandomNonce(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	encryptor, err := NewLocalEncryptor(key)
	if err != nil {
		t.Fatalf("NewLocalEncryptor() error = %v", err)
	}
	first, firstVersion, err := encryptor.Encrypt([]byte("secret-value"))
	if err != nil {
		t.Fatalf("Encrypt(first) error = %v", err)
	}
	second, secondVersion, err := encryptor.Encrypt([]byte("secret-value"))
	if err != nil {
		t.Fatalf("Encrypt(second) error = %v", err)
	}
	if firstVersion != "local-v1" || secondVersion != "local-v1" {
		t.Fatalf("key versions = %q, %q", firstVersion, secondVersion)
	}
	if bytes.Equal(first, second) {
		t.Fatal("same plaintext produced identical ciphertext")
	}
	if bytes.Contains(first, []byte("secret-value")) {
		t.Fatal("ciphertext contains plaintext")
	}
}

func TestFromEnvironmentRequiresProductionKey(t *testing.T) {
	t.Setenv("OPSK_CREDENTIAL_KEY", "")
	if _, err := FromEnvironment("production"); err == nil {
		t.Fatal("FromEnvironment(production) error = nil")
	}
	key := bytes.Repeat([]byte{9}, 32)
	t.Setenv("OPSK_CREDENTIAL_KEY", base64.StdEncoding.EncodeToString(key))
	if _, err := FromEnvironment("production"); err != nil {
		t.Fatalf("FromEnvironment(production) error = %v", err)
	}
}
