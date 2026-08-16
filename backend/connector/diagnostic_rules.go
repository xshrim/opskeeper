package connector

// EvaluateDiagnosticSnapshot turns fixed, typed connector facts into stable
// findings. Protocol adapters collect facts; this function owns thresholds and
// never uses model output. It is called before a snapshot becomes Evidence.
func EvaluateDiagnosticSnapshot(snapshot DiagnosticSnapshot) []Finding {
	result := append([]Finding(nil), snapshot.Findings...)
	switch snapshot.Kind {
	case "PostgreSQL":
		if diagnosticNumber(snapshot.Facts["waiting_locks"]) > 0 {
			result = append(result, Finding{Code: "postgresql.waiting_locks", Severity: "warning", Message: "存在等待中的数据库锁"})
		}
		if diagnosticNumber(snapshot.Facts["long_running_queries"]) > 0 {
			result = append(result, Finding{Code: "postgresql.long_running_queries", Severity: "warning", Message: "存在执行超过 5 秒的活跃查询"})
		}
	case "Redis":
		if diagnosticNumber(snapshot.Facts["rejected_connections"]) > 0 {
			result = append(result, Finding{Code: "redis.rejected_connections", Severity: "warning", Message: "Redis 曾拒绝客户端连接"})
		}
		if diagnosticNumber(snapshot.Facts["slowlog_entries"]) > 0 {
			result = append(result, Finding{Code: "redis.slowlog", Severity: "warning", Message: "存在近期慢命令记录"})
		}
	case "Kafka":
		if diagnosticNumber(snapshot.Facts["under_replicated_partitions"]) > 0 {
			result = append(result, Finding{Code: "kafka.under_replicated", Severity: "warning", Message: "存在 ISR 不完整分区"})
		}
		if diagnosticNumber(snapshot.Facts["offline_replicas"]) > 0 {
			result = append(result, Finding{Code: "kafka.offline_replicas", Severity: "critical", Message: "存在离线副本"})
		}
		if diagnosticNumber(snapshot.Facts["consumer_lag"]) > 0 {
			result = append(result, Finding{Code: "kafka.consumer_lag", Severity: "warning", Message: "存在消费者积压"})
		}
	}
	return uniqueDiagnosticFindings(result)
}

func diagnosticNumber(value any) int64 {
	switch item := value.(type) {
	case int:
		return int64(item)
	case int64:
		return item
	case float64:
		return int64(item)
	default:
		return 0
	}
}

func uniqueDiagnosticFindings(items []Finding) []Finding {
	seen := map[string]bool{}
	result := make([]Finding, 0, len(items))
	for _, item := range items {
		if !seen[item.Code] {
			seen[item.Code] = true
			result = append(result, item)
		}
	}
	return result
}
