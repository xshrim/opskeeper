package notification

import (
	"strings"
	"testing"
)

func TestFindingTransition(t *testing.T) {
	resolved := FindingState{Status: "resolved", Severity: "warning"}
	openWarning := FindingState{Status: "open", Severity: "warning"}
	tests := []struct {
		name     string
		previous *FindingState
		current  FindingState
		want     EventType
	}{
		{name: "opened", current: openWarning, want: FindingOpened},
		{name: "reopened", previous: &resolved, current: openWarning, want: FindingReopened},
		{name: "severity changed", previous: &openWarning, current: FindingState{Status: "open", Severity: "critical"}, want: FindingSeverityChanged},
		{name: "unchanged", previous: &openWarning, current: openWarning},
		{name: "resolved", previous: &openWarning, current: FindingState{Status: "resolved", Severity: "warning"}, want: FindingResolved},
		{name: "already resolved", previous: &resolved, current: resolved},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FindingTransition(test.previous, test.current); got != test.want {
				t.Fatalf("FindingTransition() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSanitizeSummary(t *testing.T) {
	got := SanitizeSummary("request https://user:pass@example.test failed; token=abc Bearer xyz password:secret")
	for _, secret := range []string{"user:pass", "token=abc", "Bearer xyz", "password:secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("SanitizeSummary() leaked %q in %q", secret, got)
		}
	}
	if got := []rune(SanitizeSummary(strings.Repeat("x", 600))); len(got) != 500 {
		t.Fatalf("SanitizeSummary() rune count = %d, want 500", len(got))
	}
}
