package notification

import (
	"regexp"
	"strings"
)

var (
	credentialAssignment = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|authorization)(\s*[:=]\s*)([^\s&;,]+)`)
	credentialURL        = regexp.MustCompile(`(?i)(https?://)[^/\s:@]+(?::[^/\s@]*)?@`)
	credentialAuth       = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[a-z0-9+/._=-]+`)
)

type EventType string

const (
	FindingOpened          EventType = "finding.opened"
	FindingReopened        EventType = "finding.reopened"
	FindingSeverityChanged EventType = "finding.severity_changed"
	FindingResolved        EventType = "finding.resolved"
	InspectionFailed       EventType = "inspection.failed"
	InspectionDegraded     EventType = "inspection.degraded"
)

type FindingState struct {
	Status   string
	Severity string
}

func FindingTransition(previous *FindingState, current FindingState) EventType {
	if current.Status == "resolved" {
		if previous != nil && previous.Status == "open" {
			return FindingResolved
		}
		return ""
	}
	if current.Status != "open" {
		return ""
	}
	if previous == nil {
		return FindingOpened
	}
	if previous.Status == "resolved" {
		return FindingReopened
	}
	if previous.Status == "open" && previous.Severity != current.Severity {
		return FindingSeverityChanged
	}
	return ""
}

func SanitizeSummary(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, value)
	value = credentialURL.ReplaceAllString(value, `${1}[redacted]@`)
	value = credentialAuth.ReplaceAllString(value, `$1 [redacted]`)
	value = credentialAssignment.ReplaceAllString(value, `$1$2[redacted]`)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 500 {
		value = string(runes[:500])
	}
	return value
}
