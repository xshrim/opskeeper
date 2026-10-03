package notification

import (
	"testing"
	"time"
)

func TestValidateRuleAndMatchesFilters(t *testing.T) {
	rule, err := ValidateRule(Rule{
		Name: "  Production alerts ", EventTypes: []EventType{FindingOpened, FindingOpened}, MinimumSeverity: "warning",
		Filters: RuleFilters{ResourceIDs: []string{"resource-1"}, FindingRules: []string{"disk.capacity"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rule.Name != "Production alerts" || len(rule.EventTypes) != 1 {
		t.Fatalf("normalized rule = %#v", rule)
	}
	if !RuleMatches(rule, RuleEvent{Type: FindingOpened, Severity: "critical", ResourceID: "resource-1", FindingRule: "disk.capacity"}) {
		t.Fatal("matching event was rejected")
	}
	for _, event := range []RuleEvent{
		{Type: FindingResolved, Severity: "critical", ResourceID: "resource-1", FindingRule: "disk.capacity"},
		{Type: FindingOpened, Severity: "info", ResourceID: "resource-1", FindingRule: "disk.capacity"},
		{Type: FindingOpened, Severity: "critical", ResourceID: "other", FindingRule: "disk.capacity"},
		{Type: FindingOpened, Severity: "critical", ResourceID: "resource-1", FindingRule: "other"},
	} {
		if RuleMatches(rule, event) {
			t.Errorf("nonmatching event was accepted: %+v", event)
		}
	}
	if _, err := ValidateRule(Rule{Name: "x", EventTypes: []EventType{"unexpected"}}); err == nil {
		t.Fatal("unknown event type was accepted")
	}
}

func TestNextAllowedAtHandlesTimezoneAndOvernightWindow(t *testing.T) {
	window := SilenceWindow{Timezone: "Asia/Shanghai", Start: "22:00", End: "07:00", Weekdays: []time.Weekday{time.Friday}}
	input := time.Date(2026, time.October, 2, 23, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	allowed, err := NextAllowedAt(window, input)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.October, 3, 7, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)).UTC()
	if !allowed.Equal(want) {
		t.Fatalf("NextAllowedAt() = %s, want %s", allowed, want)
	}
	input = time.Date(2026, time.October, 3, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	allowed, err = NextAllowedAt(window, input)
	if err != nil || !allowed.Equal(want) {
		t.Fatalf("overnight carry NextAllowedAt() = %s, %v; want %s", allowed, err, want)
	}
}
