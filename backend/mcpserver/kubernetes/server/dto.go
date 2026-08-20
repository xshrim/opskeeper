package server

type ResourceItem struct {
	Namespace string            `json:"namespace,omitempty"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind,omitempty"`
	Phase     string            `json:"phase,omitempty"`
	Ready     bool              `json:"ready,omitempty"`
	Restarts  int32             `json:"restarts,omitempty"`
	Node      string            `json:"node,omitempty"`
	Age       string            `json:"age,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}
type ListOutput struct {
	Items     []ResourceItem `json:"items"`
	Count     int            `json:"count"`
	Truncated bool           `json:"truncated"`
	Continue  string         `json:"continue,omitempty"`
}
type DetailOutput struct {
	Item map[string]any `json:"item"`
}
type LogsOutput struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container,omitempty"`
	Logs      string `json:"logs"`
	Truncated bool   `json:"truncated"`
}
type ClusterInfoOutput struct {
	Version map[string]any `json:"version,omitempty"`
	Profile string         `json:"profile,omitempty"`
	Server  string         `json:"server,omitempty"`
}
type APIResourcesOutput struct {
	Resources []map[string]any `json:"resources"`
}
type HealthOutput struct {
	Healthy bool              `json:"healthy"`
	Checks  map[string]string `json:"checks"`
}
