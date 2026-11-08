package diagnosis

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// applicationResourceFocus is a compact index of the selected applications'
// resource relationships. application_context remains the complete immutable
// snapshot; this index is deliberately shaped for choosing a resource tool
// and its narrowest supported filter.
type applicationResourceFocus struct {
	// ResourceID is an internal join key. The model selects the already-bound
	// tool by name and must not receive database identities as evidence.
	ResourceID   string                        `json:"-"`
	ResourceName string                        `json:"resource_name,omitempty"`
	ResourceKind string                        `json:"resource_kind,omitempty"`
	Relations    []applicationResourceRelation `json:"relations"`
}

type applicationResourceRelation struct {
	ApplicationID   string         `json:"-"`
	ApplicationName string         `json:"application_name,omitempty"`
	ProjectID       string         `json:"-"`
	ProjectName     string         `json:"project_name,omitempty"`
	Relation        string         `json:"relation"`
	RuntimeKind     string         `json:"runtime_kind,omitempty"`
	Status          string         `json:"status,omitempty"`
	Binding         map[string]any `json:"binding,omitempty"`
}

type applicationResourceFocusInput struct {
	Resources []applicationResourceFocus `json:"resources"`
}

// sanitizeModelJSON removes database identities from persisted context before
// it is serialized into a model prompt. The typed diagnostic DTOs prevent IDs
// for newly-created sessions; this recursive pass also protects sessions
// created before that contract was tightened.
func sanitizeModelJSON(raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return stripInternalIDs(value), nil
}

func stripInternalIDs(value any) any {
	switch typed := value.(type) {
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, stripInternalIDs(item))
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if isInternalIDField(key) {
				continue
			}
			result[key] = stripInternalIDs(item)
		}
		return result
	default:
		return value
	}
}

func isInternalIDField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "id", "application_id", "application_ids", "project_id", "project_ids", "resource_id", "resource_ids", "target_resource_id", "target_resource_ids", "source_resource_id", "source_resource_ids", "instance_id", "instance_ids", "dependency_id", "dependency_ids", "relation_id", "relation_ids", "scope_id", "scope_ids":
		return true
	default:
		return false
	}
}

type rawApplicationResourceContext struct {
	Applications []rawApplicationResourceApplication `json:"applications"`
}

type rawApplicationResourceApplication struct {
	Application rawApplicationInfo `json:"application"`
	Project     rawProjectInfo     `json:"project"`
	Resources   []rawResourceInfo  `json:"resources"`
}

type rawApplicationInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type rawProjectInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type rawResourceInfo struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Kind        string         `json:"kind"`
	Relation    string         `json:"relation"`
	RuntimeKind string         `json:"runtime_kind,omitempty"`
	Status      string         `json:"status"`
	Binding     map[string]any `json:"binding,omitempty"`
}

// buildApplicationLocatorHints creates a server-only mapping from the
// database resource identity to the concrete object selectors configured on
// an application instance. It is used to correct omitted or placeholder
// model arguments immediately before a tool call; it is never model input.
func buildApplicationLocatorHints(raw json.RawMessage) map[string]map[string]any {
	var snapshot rawApplicationResourceContext
	if len(raw) == 0 || string(raw) == "null" || json.Unmarshal(raw, &snapshot) != nil {
		return nil
	}
	hints := make(map[string]map[string]any)
	conflicts := make(map[string]map[string]struct{})
	for _, selected := range snapshot.Applications {
		for _, resource := range selected.Resources {
			resourceID := strings.TrimSpace(resource.ID)
			if resourceID == "" || len(resource.Binding) == 0 {
				continue
			}
			if hints[resourceID] == nil {
				hints[resourceID] = make(map[string]any)
			}
			for key, value := range resource.Binding {
				if conflicts[resourceID] != nil {
					if _, conflicted := conflicts[resourceID][key]; conflicted {
						continue
					}
				}
				current, exists := hints[resourceID][key]
				if !exists || strings.TrimSpace(fmt.Sprint(current)) == "" {
					hints[resourceID][key] = value
					continue
				}
				// One runtime resource can host several application instances. Do
				// not silently choose the first selector when bindings disagree;
				// a wrong container/workload is worse than an explicit retry.
				if fmt.Sprint(current) != fmt.Sprint(value) {
					delete(hints[resourceID], key)
					if conflicts[resourceID] == nil {
						conflicts[resourceID] = make(map[string]struct{})
					}
					conflicts[resourceID][key] = struct{}{}
				}
			}
		}
	}
	if len(hints) == 0 {
		return nil
	}
	return hints
}

func buildApplicationResourceFocus(raw json.RawMessage) applicationResourceFocusInput {
	var snapshot rawApplicationResourceContext
	if len(raw) == 0 || string(raw) == "null" || json.Unmarshal(raw, &snapshot) != nil {
		return applicationResourceFocusInput{}
	}

	byResource := make(map[string]*applicationResourceFocus)
	for _, selected := range snapshot.Applications {
		for _, resource := range selected.Resources {
			resourceID := strings.TrimSpace(resource.ID)
			if resourceID == "" {
				continue
			}
			focus, ok := byResource[resourceID]
			if !ok {
				focus = &applicationResourceFocus{ResourceID: resourceID, ResourceName: resource.Name, ResourceKind: resource.Kind}
				byResource[resourceID] = focus
			} else {
				// Prefer the first non-empty metadata when a snapshot contains
				// the same resource through multiple applications.
				if focus.ResourceName == "" {
					focus.ResourceName = resource.Name
				}
				if focus.ResourceKind == "" {
					focus.ResourceKind = resource.Kind
				}
			}
			focus.Relations = append(focus.Relations, applicationResourceRelation{
				ApplicationID: selected.Application.ID, ApplicationName: selected.Application.Name,
				ProjectID: selected.Project.ID, ProjectName: selected.Project.Name,
				Relation: resource.Relation, RuntimeKind: resource.RuntimeKind,
				Status: resource.Status, Binding: resource.Binding,
			})
		}
	}

	result := applicationResourceFocusInput{Resources: make([]applicationResourceFocus, 0, len(byResource))}
	for _, focus := range byResource {
		sort.SliceStable(focus.Relations, func(i, j int) bool {
			left, right := focus.Relations[i], focus.Relations[j]
			if left.ApplicationID != right.ApplicationID {
				return left.ApplicationID < right.ApplicationID
			}
			if left.Relation != right.Relation {
				return left.Relation < right.Relation
			}
			return left.Status < right.Status
		})
		result.Resources = append(result.Resources, *focus)
	}
	sort.SliceStable(result.Resources, func(i, j int) bool {
		return result.Resources[i].ResourceID < result.Resources[j].ResourceID
	})
	return result
}
