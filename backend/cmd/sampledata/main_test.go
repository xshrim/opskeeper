package main

import (
	"strings"
	"testing"
)

func TestSamplePasswordFromEnv(t *testing.T) {
	t.Run("reads configured password", func(t *testing.T) {
		t.Setenv("OPSK_TEST_USER_PASWORD", " Sample123! ")
		password, err := samplePasswordFromEnv()
		if err != nil || password != " Sample123! " {
			t.Fatalf("samplePasswordFromEnv() = (%q, %v), want configured password", password, err)
		}
	})

	t.Run("rejects empty password", func(t *testing.T) {
		t.Setenv("OPSK_TEST_USER_PASWORD", "   ")
		_, err := samplePasswordFromEnv()
		if err == nil || !strings.Contains(err.Error(), "OPSK_TEST_USER_PASWORD") {
			t.Fatalf("samplePasswordFromEnv() error = %v, want missing variable error", err)
		}
	})
}

func TestSampleSettingsFromEnv(t *testing.T) {
	t.Setenv("OPSK_TEST_DOCKER_HOST_URL", "tcp://docker.example:2376")
	t.Setenv("OPSK_TEST_HOST_ADDRESS", "host.example")
	t.Setenv("OPSK_TEST_HOST_PORT", "2222")
	t.Setenv("OPSK_TEST_HOST_USERNAME", "sample")
	t.Setenv("OPSK_TEST_HOST_PASSWORD", "secret")
	t.Setenv("OPSK_TEST_LLM_API_KEY", "llm-secret")
	t.Setenv("OPSK_TEST_LLM_SCENARIO", "通用")

	settings, err := sampleSettingsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if settings.dockerConfig["host"] != "tcp://docker.example:2376" || settings.dockerConfig["timeout"] != 10 || settings.dockerConfig["skip_tls_verify"] != false {
		t.Errorf("Docker settings = %#v", settings.dockerConfig)
	}
	if settings.hostConfig["host"] != "host.example" || settings.hostConfig["port"] != 2222 || settings.hostConfig["auth_method"] != "password" {
		t.Errorf("Host settings = %#v", settings.hostConfig)
	}
	if settings.hostCredential["password"] != "secret" || len(settings.hostCredential) != 1 || settings.llmAPIKey != "llm-secret" {
		t.Errorf("Host/LLM credentials were not loaded")
	}
	if settings.llmScenario != "general" {
		t.Errorf("LLM scenario = %q, want general", settings.llmScenario)
	}
}

func TestEnsureLocalDatabase(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		url         string
		wantErr     bool
	}{
		{name: "local development database", environment: "development", url: "postgres://user:pass@localhost:5432/opskeeper?sslmode=disable"},
		{name: "loopback address", environment: "development", url: "postgres://user:pass@127.0.0.1:5432/opskeeper"},
		{name: "production environment", environment: "production", url: "postgres://user:pass@localhost:5432/opskeeper", wantErr: true},
		{name: "remote host", environment: "development", url: "postgres://user:pass@db.example.com:5432/opskeeper", wantErr: true},
		{name: "other database", environment: "development", url: "postgres://user:pass@localhost:5432/production", wantErr: true},
		{name: "invalid URL", environment: "development", url: "not a database URL", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ensureLocalDatabase(test.environment, test.url)
			if (err != nil) != test.wantErr {
				t.Fatalf("ensureLocalDatabase() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestProjectScopeKeyUsesOneBasedProjectNumbers(t *testing.T) {
	for _, test := range []struct {
		teamKey string
		index   int
		want    string
	}{
		{teamKey: "beta", index: 0, want: "beta-01"},
		{teamKey: "beta", index: 1, want: "beta-02"},
	} {
		if got := projectScopeKey(test.teamKey, test.index); got != test.want {
			t.Errorf("projectScopeKey(%q, %d) = %q, want %q", test.teamKey, test.index, got, test.want)
		}
	}
}
