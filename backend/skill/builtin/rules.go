package builtin

import "opskeeper/backend/connector"

// Evaluate applies deterministic, version-independent thresholds to a bounded
// Connector snapshot. The result is intentionally separate from model text.
func Evaluate(snapshot connector.DiagnosticSnapshot) []connector.Finding {
	return connector.EvaluateDiagnosticSnapshot(snapshot)
}
