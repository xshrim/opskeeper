package inspection

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// FindingFingerprint provides stable deduplication within a scheduling window.
// It intentionally excludes message text so wording changes cannot reopen an
// already-observed deterministic rule violation.
func FindingFingerprint(targetResourceID, rule string, windowStart time.Time) string {
	value := strings.Join([]string{strings.TrimSpace(targetResourceID), strings.TrimSpace(rule), windowStart.UTC().Format(time.RFC3339)}, "\x00")
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// FindingIdentityKey is deliberately independent of the scheduling window.
// It lets an existing finding remain open across observations and be reopened
// after a recovery, while FindingFingerprint deduplicates events per window.
func FindingIdentityKey(targetResourceID, rule string) string {
	value := strings.Join([]string{strings.TrimSpace(targetResourceID), strings.TrimSpace(rule)}, "\x00")
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// HealthScore is deterministic. AI may explain reasons but cannot influence
// score calculation. Severity weights are summed and clamped to [0,100].
func HealthScore(results []RuleResult) int {
	penalty := 0
	for _, result := range results {
		weight := result.Weight
		if weight <= 0 {
			switch result.Severity {
			case "critical":
				weight = 50
			case "warning":
				weight = 20
			default:
				weight = 5
			}
		}
		penalty += weight
	}
	if penalty > 100 {
		return 0
	}
	return 100 - penalty
}

func IsMaintenance(window []MaintenanceWindow, at time.Time) bool {
	for _, item := range window {
		if !item.Start.IsZero() && !item.End.IsZero() && !at.Before(item.Start) && at.Before(item.End) {
			return true
		}
	}
	return false
}

func distinctIDs(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
