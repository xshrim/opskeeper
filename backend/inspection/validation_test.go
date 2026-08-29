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
