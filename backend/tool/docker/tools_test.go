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
