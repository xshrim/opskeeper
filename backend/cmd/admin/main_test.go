package main

import (
	"os"
	"strings"
	"testing"
)

func TestParseCreateOptions(t *testing.T) {
	options, err := parseCreateOptions([]string{
		"--username", "admin",
		"--email", "admin@example.com",
		"--phone", "+8613800138000",
		"--display-name=Platform Admin",
		"--password", "password-from-flag",
		"--password-file", "/run/secrets/admin-password",
		"--if-needed",
	}, &strings.Builder{})
	if err != nil {
		t.Fatalf("parseCreateOptions() error = %v", err)
	}
	if options.username != "admin" || options.email != "admin@example.com" || options.phone != "+8613800138000" || options.displayName != "Platform Admin" || options.password != "password-from-flag" || options.passwordFile != "/run/secrets/admin-password" || !options.ifNeeded {
		t.Fatalf("parseCreateOptions() = %#v", options)
	}
}

func TestParseCreateOptionsRejectsUnexpectedArguments(t *testing.T) {
	if _, err := parseCreateOptions([]string{"extra"}, &strings.Builder{}); err == nil {
		t.Fatal("parseCreateOptions() error = nil, want unexpected argument error")
	}
}

func TestLoadPasswordGeneratesURLSafePassword(t *testing.T) {
	password, generated, err := loadPassword("", "")
	if err != nil {
		t.Fatalf("loadPassword() error = %v", err)
	}
	if !generated || len(password) != 32 {
		t.Fatalf("loadPassword() = (%q, %t), want a generated 32-character password", password, generated)
	}
	if strings.ContainsAny(password, "+/=\r\n") {
		t.Fatalf("generated password contains unsafe characters: %q", password)
	}
}

func TestLoadPasswordReadsPasswordFile(t *testing.T) {
	file := t.TempDir() + "/password"
	if err := os.WriteFile(file, []byte("password-from-file\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	password, generated, err := loadPassword("", file)
	if err != nil {
		t.Fatalf("loadPassword() error = %v", err)
	}
	if generated || password != "password-from-file" {
		t.Fatalf("loadPassword() = (%q, %t)", password, generated)
	}
}

func TestLoadPasswordUsesCommandLinePassword(t *testing.T) {
	password, generated, err := loadPassword("password-from-flag", "/does/not/exist")
	if err != nil {
		t.Fatalf("loadPassword() error = %v", err)
	}
	if generated || password != "password-from-flag" {
		t.Fatalf("loadPassword() = (%q, %t)", password, generated)
	}
}
