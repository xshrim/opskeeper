package diagnosis

import (
	"encoding/json"
	"sort"
	"strings"
)

// applicationResourceFocus is a compact index of the selected applications'
// resource relationships. application_context remains the complete immutable
// snapshot; this index is deliberately shaped for choosing a resource tool
// and its narrowest supported filter.
type applicationResourceFocus struct {
	ResourceID   string                        `json:"resource_id"`
	ResourceName string                        `json:"resource_name,omitempty"`
	ResourceKind string                        `json:"resource_kind,omitempty"`
	Relations    []applicationResourceRelation `json:"relations"`
}

type applicationResourceRelation struct {
	ApplicationID   string         `json:"application_id"`
	ApplicationName string         `json:"application_name,omitempty"`
	ProjectID       string         `json:"project_id,omitempty"`
	ProjectName     string         `json:"project_name,omitempty"`
	Relation        string         `json:"relation"`
	RuntimeKind     string         `json:"runtime_kind,omitempty"`
	Status          string         `json:"status,omitempty"`
	Binding         map[string]any `json:"binding,omitempty"`
}

type applicationResourceFocusInput struct {
	Resources []applicationResourceFocus `json:"resources"`
}

type diagnosticApplicationContext struct {
	Applications []diagnosticApplication `json:"applications"`
}

type diagnosticApplication struct {
	Application diagnosticApplicationInfo `json:"application"`
	Project     diagnosticProjectInfo     `json:"project"`
	Resources   []diagnosticResourceInfo  `json:"resources"`
}

type diagnosticApplicationInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type diagnosticProjectInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type diagnosticResourceInfo struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Kind        string         `json:"kind"`
	Relation    string         `json:"relation"`
	RuntimeKind string         `json:"runtime_kind,omitempty"`
	Status      string         `json:"status"`
	Binding     map[string]any `json:"binding,omitempty"`
}

func buildApplicationResourceFocus(raw json.RawMessage) applicationResourceFocusInput {
	var snapshot diagnosticApplicationContext
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
