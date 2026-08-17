package inspection

import (
	"slices"
	"strings"
	"time"
)

func normalizePolicy(input Policy) (Policy, error) {
	input.ID, input.ScopeID, input.Name = strings.TrimSpace(input.ID), strings.TrimSpace(input.ScopeID), strings.TrimSpace(input.Name)
	input.Cron, input.Timezone, input.Status = strings.TrimSpace(input.Cron), strings.TrimSpace(input.Timezone), strings.TrimSpace(input.Status)
	if input.ScopeID == "" || input.Name == "" || len([]rune(input.Name)) > 200 {
		return Policy{}, invalid("policy scope_id and name are required; name must not exceed 200 characters")
	}
	if input.Cron == "" || input.Timezone == "" {
		return Policy{}, invalid("policy cron and timezone are required")
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return Policy{}, invalid("policy timezone is invalid")
	}
	if input.Status == "" {
		input.Status = PolicyActive
	}
	if input.Status != PolicyActive && input.Status != PolicyDisabled {
		return Policy{}, invalid("policy status must be active or disabled")
	}
	input.TargetResourceIDs, input.SkillResourceIDs = distinctIDs(input.TargetResourceIDs), distinctIDs(input.SkillResourceIDs)
	if len(input.TargetResourceIDs) == 0 && len(input.TargetLabels) == 0 {
		return Policy{}, invalid("policy requires target_resource_ids or target_labels")
	}
	if len(input.SkillResourceIDs) == 0 {
		return Policy{}, invalid("policy requires at least one Skill")
	}
	if input.Timeout <= 0 || input.Timeout > time.Hour {
		return Policy{}, invalid("policy timeout must be between 1 second and 1 hour")
	}
	input.TimeoutSeconds = int(input.Timeout / time.Second)
	if input.Retries < 0 || input.Retries > 10 || input.MaxConcurrent < 1 || input.MaxConcurrent > 64 || input.MaxToolCalls < 1 || input.MaxToolCalls > 100 || input.MaxTokens < 1 || input.MaxTokens > 200000 {
		return Policy{}, invalid("policy retries, concurrency or budget is out of range")
	}
	for _, window := range input.Maintenance {
		if window.Start.IsZero() || window.End.IsZero() || !window.End.After(window.Start) {
			return Policy{}, invalid("maintenance window must have a positive start/end range")
		}
	}
	return input, nil
}

func policyHasSkill(policy Policy, skillID string) bool {
	return slices.Contains(policy.SkillResourceIDs, skillID)
}
