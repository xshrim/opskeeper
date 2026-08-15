package identity

import (
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	if !verifyPassword(hash, "correct horse battery staple") {
		t.Fatal("verifyPassword() = false for correct password")
	}
	if verifyPassword(hash, "wrong password") {
		t.Fatal("verifyPassword() = true for wrong password")
	}
	if passwordNeedsUpgrade(hash) {
		t.Fatal("passwordNeedsUpgrade() = true for current parameters")
	}
}

func TestPasswordHashRejectsMalformedAndExcessiveParameters(t *testing.T) {
	for _, hash := range []string{
		"not-a-password-hash",
		"$argon2id$v=19$m=999999999,t=3,p=2$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=99,p=2$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
	} {
		if verifyPassword(hash, "password") {
			t.Fatalf("verifyPassword(%q) = true", hash)
		}
	}
}

func TestPasswordHashDetectsOlderParameters(t *testing.T) {
	salt := []byte("0123456789abcdef")
	hash := argon2.IDKey([]byte("upgrade me please"), salt, 2, 32*1024, 1, 32)
	encoded := "$argon2id$v=19$m=32768,t=2,p=1$" +
		base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash)
	if !verifyPassword(encoded, "upgrade me please") {
		t.Fatal("verifyPassword() = false for supported older parameters")
	}
	if !passwordNeedsUpgrade(encoded) {
		t.Fatal("passwordNeedsUpgrade() = false for older parameters")
	}
}
