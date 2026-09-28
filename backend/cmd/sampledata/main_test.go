package main

import "testing"

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
