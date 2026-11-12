package inspection

import (
	"errors"
	"testing"
	"time"
)

func TestNormalizePolicyRequiresBoundedAuthorizedIntent(t *testing.T) {
	base := Policy{ScopeID: "scope-1", Name: "API health", Cron: "0 * * * *", Timezone: "Asia/Shanghai", TargetResourceIDs: []string{"target-1", "target-1"}, Timeout: time.Minute, MaxConcurrent: 1, MaxToolCalls: 12, MaxTokens: 20000}
	policy, err := normalizePolicy(base)
	if err != nil {
		t.Fatalf("normalizePolicy() error = %v", err)
	}
	if policy.Status != PolicyActive || len(policy.TargetResourceIDs) != 1 {
		t.Fatalf("normalized policy = %#v", policy)
	}
	base.TargetResourceIDs, base.TargetLabels = nil, nil
	if _, err := normalizePolicy(base); !errors.As(err, new(*ValidationError)) {
		t.Fatalf("missing target error = %v", err)
	}
}

func TestNormalizePolicyAllowsApplicationTargets(t *testing.T) {
	policy, err := normalizePolicy(Policy{
		ScopeID: "scope-1", Name: "application health", Cron: "0 * * * *", Timezone: "UTC",
		TargetApplicationIDs: []string{"app-1", "app-1"}, Timeout: time.Minute,
		MaxConcurrent: 1, MaxToolCalls: 12, MaxTokens: 20000,
	})
	if err != nil {
		t.Fatalf("normalizePolicy() error = %v", err)
	}
	if len(policy.TargetApplicationIDs) != 1 || policy.TargetApplicationIDs[0] != "app-1" {
		t.Fatalf("application targets = %v, want one distinct app", policy.TargetApplicationIDs)
	}
}
