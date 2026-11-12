package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DescribeToolParameters preserves provider descriptions and supplies model-
// facing meaning for schema fields that would otherwise be ambiguous.
func DescribeToolParameters(resourceKind string, raw json.RawMessage) json.RawMessage {
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return raw
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return raw
	}
	for key, value := range properties {
		property, ok := value.(map[string]any)
		description, _ := property["description"].(string)
		if !ok || strings.TrimSpace(description) != "" {
			continue
		}
		property["description"] = toolParameterDescription(resourceKind, key)
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return encoded
}

func toolParameterDescription(resourceKind, key string) string {
	if strings.EqualFold(resourceKind, "Repository") {
		descriptions := map[string]string{
			"repository_path":    "Path relative to the selected repository root; omit to search or inventory the whole repository.",
			"file_path":          "Tracked UTF-8 file path relative to the selected repository root.",
			"search_pattern":     "Text pattern to find in tracked files; interpreted as a grep -E expression only when regular_expression is true.",
			"regular_expression": "When true, interpret search_pattern with grep -E extended regular-expression syntax; otherwise use literal matching.",
			"ignore_case":        "When true, ignore letter case, equivalent to grep -i.",
			"start_offset":       "Zero-based result index for the next page; use the previous response's next_offset.",
			"byte_offset":        "Zero-based byte position in the file; use the previous response's next_offset to continue.",
			"page_size":          "Maximum number of file or match entries in this page, subject to the tool maximum.",
			"max_bytes":          "Maximum UTF-8 text bytes to read from this file range, up to 1 MiB.",
			"revision":           "Branch, tag, or commit to inspect; omit to use the execution-pinned revision.",
		}
		if description := descriptions[key]; description != "" {
			return description
		}
	}
	if key == "query" {
		switch strings.ToLower(resourceKind) {
		case "prometheus":
			return "PromQL expression used to select the metric series and values to inspect."
		case "loki":
			return "LogQL expression used to select the log streams and entries to inspect."
		case "postgresql", "mysql", "oracle", "oceanbase":
			return "Read-only SQL statement to explain or inspect; write statements are not allowed."
		}
	}
	if key == "path" && strings.EqualFold(resourceKind, "Repository") {
		return "Path within the selected Git repository, relative to its root; omit to use the repository root."
	}
	if key == "path" && strings.EqualFold(resourceKind, "Docker") {
		return "Absolute path of a regular file inside the selected Docker container."
	}
	descriptions := map[string]string{
		"namespace":      "Namespace containing the Kubernetes object; omit only when the tool supports all namespaces.",
		"workload_kind":  "Kubernetes workload kind, such as Deployment, StatefulSet, DaemonSet, Job, or CronJob.",
		"workload_name":  "Exact Kubernetes workload name in the specified namespace.",
		"pod":            "Exact Kubernetes Pod name in the specified namespace.",
		"container_name": "Exact container name inside the selected Docker container or Kubernetes Pod; not a container ID.",
		"object_name":    "Exact name of the Kubernetes API object to retrieve; not an OpsKeeper database ID.",
		"resource":       "Kubernetes API resource type, such as pods, deployments, or services; not an OpsKeeper resource ID.",
		"path":           "Resource-relative or absolute path as required by this tool; do not pass an OpsKeeper database ID.",
		"revision":       "Git branch, tag, or commit revision to inspect; omit to use the configured revision.",
		"offset":         "Zero-based offset for continuing a bounded result page or file read.",
		"limit":          "Maximum number of results to return; subject to the tool's documented maximum.",
		"query":          "Query expression in the syntax accepted by this resource tool.",
		"schema":         "Database schema containing the requested table or object.",
		"table":          "Exact database table name within the specified schema.",
		"index":          "Exact Elasticsearch index name whose mapping is requested.",
		"topic":          "Exact Kafka topic name to inspect.",
		"bucket":         "Exact object-storage bucket name.",
		"object":         "Exact object key within the specified bucket.",
		"prefix":         "Optional object-key prefix used to narrow the listing.",
		"service_name":   "Exact service name to inspect in the configured service registry.",
		"data_id":        "Exact configuration data ID to retrieve from the configuration center.",
		"group_name":     "Registry or configuration group name; omit to use the service default.",
		"keyword":        "Name or content filter interpreted by this tool; use the documented matching syntax.",
		"filters":        "Resource-specific filter expression; use the syntax documented by this tool.",
		"start":          "Inclusive start time for the requested metric or log interval, in the documented timestamp format.",
		"end":            "Exclusive end time for the requested metric or log interval, in the documented timestamp format.",
		"step_seconds":   "Metric sampling interval in seconds.",
		"min_total_ms":   "Minimum cumulative query duration in milliseconds used to filter results.",
		"tail":           "Maximum number of recent log lines to request, subject to the tool's output limit.",
		"since":          "Return logs at or after this timestamp or duration.",
		"until":          "Return logs up to this timestamp.",
		"pid":            "Exact operating-system process ID returned by a host process listing; never an OpsKeeper database ID.",
		"node":           "Exact Kubernetes node name returned by the cluster.",
		"continue":       "Opaque Kubernetes continuation token returned by the previous page; do not invent one.",
		"page":           "One-based result page number.",
		"page_size":      "Maximum number of results in one page.",
		"healthy_only":   "When true, return only healthy service instances.",
		"search":         "Optional substring used to filter registry or configuration entries.",
		"sample_seconds": "Sampling duration in seconds; zero requests a single current snapshot.",
		"container_id":   "Real Docker container ID returned by Docker, not a container name or an OpsKeeper database ID.",
		"timestamps":     "When true, include timestamps with returned log lines.",
		"details":        "When true, include additional metadata provided by the resource API.",
		"all":            "When true, include non-default or stopped objects where supported.",
	}
	if description := descriptions[key]; description != "" {
		return description
	}
	return fmt.Sprintf("Value used to select or filter %s data for this tool; follow the tool description and do not use an OpsKeeper database ID.", strings.ToLower(resourceKind))
}
