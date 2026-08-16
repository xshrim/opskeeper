package operation

import "testing"

func TestMediumAndHighRequireApproval(t *testing.T) {
	for _, risk := range []string{RiskMedium, RiskHigh} {
		if !RequiresApproval(risk) {
			t.Fatalf("%s must require approval", risk)
		}
	}
	for _, risk := range []string{RiskReadOnly, RiskLow} {
		if RequiresApproval(risk) {
			t.Fatalf("%s must not require approval", risk)
		}
	}
}
