package notification

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type RuleFilters struct {
	ResourceIDs  []string `json:"resource_ids,omitempty"`
	FindingRules []string `json:"finding_rules,omitempty"`
}

type SilenceWindow struct {
	Timezone string         `json:"timezone"`
	Start    string         `json:"start"`
	End      string         `json:"end"`
	Weekdays []time.Weekday `json:"weekdays,omitempty"`
}

type Rule struct {
	Name            string        `json:"name"`
	Status          string        `json:"status"`
	EventTypes      []EventType   `json:"event_types"`
	MinimumSeverity string        `json:"minimum_severity"`
	Filters         RuleFilters   `json:"filters"`
	Cooldown        time.Duration `json:"cooldown"`
	Aggregation     time.Duration `json:"aggregation"`
	MaxBatchSize    int           `json:"max_batch_size"`
	Silence         SilenceWindow `json:"silence"`
}

type RuleEvent struct {
	Type            EventType
	Severity        string
	ResourceID      string
	FindingRule     string
	FindingIdentity string
}

func ValidateRule(rule Rule) (Rule, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" || len([]rune(rule.Name)) > 120 {
		return Rule{}, fmt.Errorf("notification rule name must contain 1 to 120 characters")
	}
	if rule.Status == "" {
		rule.Status = "active"
	}
	if rule.Status != "active" && rule.Status != "disabled" {
		return Rule{}, fmt.Errorf("invalid notification rule status")
	}
	if len(rule.EventTypes) == 0 {
		return Rule{}, fmt.Errorf("notification rule requires an event type")
	}
	events := make(map[EventType]bool, len(rule.EventTypes))
	for _, event := range rule.EventTypes {
		switch event {
		case FindingOpened, FindingReopened, FindingSeverityChanged, FindingResolved, InspectionFailed, InspectionDegraded, EventType("notification.test"):
			events[event] = true
		default:
			return Rule{}, fmt.Errorf("unknown notification event type")
		}
	}
	rule.EventTypes = rule.EventTypes[:0]
	for event := range events {
		rule.EventTypes = append(rule.EventTypes, event)
	}
	sort.Slice(rule.EventTypes, func(i, j int) bool { return rule.EventTypes[i] < rule.EventTypes[j] })
	if rule.MinimumSeverity == "" {
		rule.MinimumSeverity = "info"
	}
	if severityRank(rule.MinimumSeverity) < 0 {
		return Rule{}, fmt.Errorf("invalid minimum severity")
	}
	if rule.Cooldown < 0 || rule.Cooldown > 30*24*time.Hour || rule.Aggregation < 0 || rule.Aggregation > 24*time.Hour {
		return Rule{}, fmt.Errorf("notification rule durations are out of range")
	}
	if rule.MaxBatchSize == 0 {
		rule.MaxBatchSize = 1
	}
	if rule.MaxBatchSize < 1 || rule.MaxBatchSize > 1000 {
		return Rule{}, fmt.Errorf("notification rule batch size must be between 1 and 1000")
	}
	if len(rule.Filters.ResourceIDs)+len(rule.Filters.FindingRules) > 500 {
		return Rule{}, fmt.Errorf("notification rule has too many filters")
	}
	for _, value := range append(append([]string{}, rule.Filters.ResourceIDs...), rule.Filters.FindingRules...) {
		if strings.TrimSpace(value) == "" || len(value) > 300 {
			return Rule{}, fmt.Errorf("notification rule filter value is invalid")
		}
	}
	if hasSilence(rule.Silence) {
		if rule.Silence.Timezone == "" || !validClock(rule.Silence.Start) || !validClock(rule.Silence.End) {
			return Rule{}, fmt.Errorf("notification silence window requires a timezone and valid start/end times")
		}
		if _, err := time.LoadLocation(rule.Silence.Timezone); err != nil {
			return Rule{}, fmt.Errorf("notification silence timezone is invalid")
		}
		for _, day := range rule.Silence.Weekdays {
			if day < time.Sunday || day > time.Saturday {
				return Rule{}, fmt.Errorf("notification silence weekday is invalid")
			}
		}
	}
	return rule, nil
}

func RuleMatches(rule Rule, event RuleEvent) bool {
	if rule.Status == "disabled" || !containsEvent(rule.EventTypes, event.Type) {
		return false
	}
	if severityRank(event.Severity) >= 0 && severityRank(event.Severity) < severityRank(rule.MinimumSeverity) {
		return false
	}
	if len(rule.Filters.ResourceIDs) > 0 && !containsString(rule.Filters.ResourceIDs, event.ResourceID) {
		return false
	}
	if len(rule.Filters.FindingRules) > 0 && !containsString(rule.Filters.FindingRules, event.FindingRule) {
		return false
	}
	return true
}

func NextAllowedAt(window SilenceWindow, now time.Time) (time.Time, error) {
	if !hasSilence(window) {
		return now, nil
	}
	checked, err := ValidateRule(Rule{Name: "silence-check", EventTypes: []EventType{FindingOpened}, Silence: window})
	if err != nil {
		return time.Time{}, err
	}
	window = checked.Silence
	location, _ := time.LoadLocation(window.Timezone)
	local := now.In(location)
	startMinute, _ := clockMinute(window.Start)
	endMinute, _ := clockMinute(window.End)
	minute := local.Hour()*60 + local.Minute()
	if startMinute == endMinute {
		return now, nil
	}
	inWindow := minute >= startMinute && minute < endMinute
	if startMinute > endMinute {
		inWindow = minute >= startMinute || minute < endMinute
	}
	day := local.Weekday()
	if startMinute > endMinute && minute < endMinute {
		day = (day + 6) % 7
	}
	if !silenceDayEnabled(window.Weekdays, day) {
		return now, nil
	}
	if !inWindow {
		return now, nil
	}
	endDate := local
	if startMinute > endMinute && minute >= startMinute {
		endDate = endDate.AddDate(0, 0, 1)
	}
	return time.Date(endDate.Year(), endDate.Month(), endDate.Day(), endMinute/60, endMinute%60, 0, 0, location).UTC(), nil
}

func DecodeRuleFilters(raw []byte) (RuleFilters, error) {
	var filters RuleFilters
	if err := json.Unmarshal(raw, &filters); err != nil {
		return RuleFilters{}, fmt.Errorf("decode notification rule filters: %w", err)
	}
	encoded, _ := json.Marshal(filters)
	var input map[string]json.RawMessage
	_ = json.Unmarshal(raw, &input)
	var normalized map[string]json.RawMessage
	_ = json.Unmarshal(encoded, &normalized)
	for key := range input {
		if _, ok := normalized[key]; !ok {
			return RuleFilters{}, fmt.Errorf("unknown notification rule filter")
		}
	}
	return filters, nil
}

func severityRank(value string) int {
	switch value {
	case "info":
		return 0
	case "warning":
		return 1
	case "critical":
		return 2
	default:
		return -1
	}
}

func validClock(value string) bool {
	_, err := clockMinute(value)
	return err == nil
}

func clockMinute(value string) (int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return 0, fmt.Errorf("invalid local time")
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func containsEvent(values []EventType, value EventType) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func hasSilence(window SilenceWindow) bool {
	return window.Timezone != "" || window.Start != "" || window.End != "" || len(window.Weekdays) > 0
}

func silenceDayEnabled(days []time.Weekday, day time.Weekday) bool {
	if len(days) == 0 {
		return true
	}
	for _, item := range days {
		if item == day {
			return true
		}
	}
	return false
}
