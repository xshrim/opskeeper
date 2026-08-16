package inspection

import (
	"context"
	"errors"
	"testing"
	"time"

	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type memoryStore struct{ policy Policy }

func (m *memoryStore) CreatePolicy(_ context.Context, policy Policy, _ string) (Policy, error) {
	policy.ID = "policy-1"
	m.policy = policy
	return policy, nil
}
func (m *memoryStore) ListPolicies(context.Context, string) ([]Policy, error) {
	return []Policy{m.policy}, nil
}
func (*memoryStore) ScheduleDue(context.Context, time.Time) (int, error) { return 0, nil }
func (*memoryStore) CreateScheduledRun(context.Context, Policy, time.Time, time.Time, []string) (string, bool, error) {
	return "", false, nil
}
func (*memoryStore) ClaimJob(context.Context, string, time.Duration) (Job, bool, error) {
	return Job{}, false, nil
}
func (*memoryStore) Heartbeat(context.Context, string, string, time.Duration) (bool, error) {
	return false, nil
}
func (*memoryStore) FinishJob(context.Context, string, string, error) error { return nil }

type policyResourceReader map[string]resource.Resource

func (r policyResourceReader) Get(_ context.Context, id string) (resource.Resource, error) {
	item, ok := r[id]
	if !ok {
		return resource.Resource{}, resource.ErrNotFound
	}
	return item, nil
}

func TestCreatePolicyRequiresAuthorizedActiveTargetAndSkill(t *testing.T) {
	store := &memoryStore{}
	resources := policyResourceReader{"target": {ID: "target", ScopeID: "scope", Kind: "PostgreSQL", Status: resource.StatusActive}, "skill": {ID: "skill", ScopeID: "scope", Kind: "Skill", Status: resource.StatusActive}}
	service := NewService(store, resources)
	ctx := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{ScopeIDs: []string{"scope"}})
	policy, err := service.CreatePolicy(ctx, Policy{ScopeID: "scope", Name: "health", Cron: "0 * * * *", Timezone: "UTC", TargetResourceIDs: []string{"target"}, SkillResourceIDs: []string{"skill"}, Timeout: time.Minute, MaxConcurrent: 1, MaxToolCalls: 1, MaxTokens: 1}, "actor")
	if err != nil || policy.ID != "policy-1" {
		t.Fatalf("CreatePolicy()=%#v,%v", policy, err)
	}
	resources["target"] = resource.Resource{ID: "target", ScopeID: "other", Kind: "PostgreSQL", Status: resource.StatusActive}
	if _, err := service.CreatePolicy(ctx, policy, "actor"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("wrong scope error=%v", err)
	}
}
