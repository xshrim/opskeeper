package inspection

import (
	"testing"
	"time"
)

func TestFindingFingerprintIsStablePerWindowAndRule(t *testing.T) {
	window := time.Date(2026, 8, 17, 1, 0, 0, 0, time.FixedZone("CST", 8*3600))
	first := FindingFingerprint("resource-1", "postgresql.waiting_locks", window)
	if first == "" || first != FindingFingerprint("resource-1", "postgresql.waiting_locks", window.UTC()) {
		t.Fatalf("fingerprint is not stable: %q", first)
	}
	if first == FindingFingerprint("resource-1", "postgresql.waiting_locks", window.Add(time.Hour)) {
		t.Fatal("different scheduling window reused a fingerprint")
	}
}

func TestHealthScoreIsDeterministicAndClamped(t *testing.T) {
	if score := HealthScore([]RuleResult{{Severity: "warning"}, {Severity: "critical"}}); score != 30 {
		t.Fatalf("score = %d, want 30", score)
	}
	if score := HealthScore([]RuleResult{{Weight: 70}, {Weight: 70}}); score != 0 {
		t.Fatalf("score = %d, want 0", score)
	}
}

func TestIsMaintenanceUsesExplicitWindow(t *testing.T) {
	start := time.Date(2026, 8, 17, 1, 0, 0, 0, time.UTC)
	if !IsMaintenance([]MaintenanceWindow{{Start: start, End: start.Add(time.Hour)}}, start.Add(30*time.Minute)) {
		t.Fatal("expected maintenance window")
	}
}
