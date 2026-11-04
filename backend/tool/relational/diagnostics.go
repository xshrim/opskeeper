package relational

// These result types are shared by PostgreSQL, MySQL, Oracle and OceanBase
// so callers can consume the same relational diagnostics across engines.
type SlowQueriesInput struct {
	Limit      int     `json:"limit,omitempty"`
	MinTotalMS float64 `json:"min_total_ms,omitempty"`
}

type SlowQuery struct {
	QueryID     string  `json:"query_id"`
	Calls       int64   `json:"calls"`
	TotalExecMS float64 `json:"total_exec_ms"`
	MeanExecMS  float64 `json:"mean_exec_ms"`
	MaxExecMS   float64 `json:"max_exec_ms,omitempty"`
	Rows        int64   `json:"rows"`
	Query       string  `json:"query"`
}

type SlowQueriesOutput struct {
	Available bool        `json:"available"`
	Reason    string      `json:"reason,omitempty"`
	Queries   []SlowQuery `json:"queries"`
}

type ActiveQuery struct {
	SessionID       int64  `json:"session_id"`
	User            string `json:"user"`
	ApplicationName string `json:"application_name"`
	ClientAddress   string `json:"client_address"`
	State           string `json:"state"`
	WaitEventType   string `json:"wait_event_type"`
	WaitEvent       string `json:"wait_event"`
	QueryStart      string `json:"query_start"`
	DurationSeconds int64  `json:"duration_seconds"`
	Query           string `json:"query"`
}

type ActiveQueriesOutput struct {
	Available bool          `json:"available"`
	Reason    string        `json:"reason,omitempty"`
	Queries   []ActiveQuery `json:"queries"`
}

type LockWait struct {
	BlockedSessionID    int64  `json:"blocked_session_id"`
	BlockingSessionID   int64  `json:"blocking_session_id"`
	BlockedUser         string `json:"blocked_user"`
	BlockingUser        string `json:"blocking_user"`
	BlockedQuery        string `json:"blocked_query"`
	BlockingQuery       string `json:"blocking_query"`
	WaitDurationSeconds int64  `json:"wait_duration_seconds"`
	WaitDurationKnown   bool   `json:"wait_duration_known"`
	LockType            string `json:"lock_type,omitempty"`
	LockResource        string `json:"lock_resource,omitempty"`
}

type LockWaitsOutput struct {
	Available bool       `json:"available"`
	Reason    string     `json:"reason,omitempty"`
	Waits     []LockWait `json:"waits"`
}

type ConnectionStatsOutput struct {
	Available         bool    `json:"available"`
	Reason            string  `json:"reason,omitempty"`
	CapacityKnown     bool    `json:"capacity_known"`
	Total             int64   `json:"total"`
	Active            int64   `json:"active"`
	Idle              int64   `json:"idle"`
	IdleInTransaction int64   `json:"idle_in_transaction"`
	Max               int64   `json:"max"`
	UtilizationPct    float64 `json:"utilization_pct"`
}

type TableConstraint struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Columns    string `json:"columns"`
	Definition string `json:"definition"`
	Status     string `json:"status"`
	Validated  string `json:"validated"`
}

type TableIndex struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Columns    string `json:"columns"`
	Definition string `json:"definition"`
	SizeBytes  int64  `json:"size_bytes"`
	IsPrimary  bool   `json:"is_primary"`
	IsUnique   bool   `json:"is_unique"`
	Status     string `json:"status"`
}

type TableStructureOutput struct {
	Schema      string            `json:"schema"`
	Table       string            `json:"table"`
	CreateSQL   string            `json:"create_sql"`
	Constraints []TableConstraint `json:"constraints"`
	Indexes     []TableIndex      `json:"indexes"`
}
