package docker

import "testing"

func TestValidateContainerPath(t *testing.T) {
	for _, path := range []string{"", "relative.log", "/var/log/../etc/passwd", "/"} {
		if _, err := validateContainerPath(path); err == nil {
			t.Errorf("validateContainerPath(%q) succeeded", path)
		}
	}
	if got, err := validateContainerPath("/var/log/app.log"); err != nil || got != "/var/log/app.log" {
		t.Fatalf("validateContainerPath = %q, %v", got, err)
	}
}

func TestResolveContainerIdentifierRejectsInternalDatabaseID(t *testing.T) {
	if _, err := resolveContainerIdentifier("251683f4-d022-4153-886a-2eec581bf110", ""); err == nil {
		t.Fatal("database UUID must not be sent to Docker")
	}
	if got, err := resolveContainerIdentifier("", "orders-api"); err != nil || got != "orders-api" {
		t.Fatalf("external container name = %q, %v", got, err)
	}
}
