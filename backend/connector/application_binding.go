package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"opskeeper/backend/engine"
	docker "opskeeper/backend/tool/docker"
	host "opskeeper/backend/tool/host"
)

type ApplicationBindingItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
	Status string `json:"status,omitempty"`
}

type ApplicationBindingPreview struct {
	Items []ApplicationBindingItem `json:"items"`
	Count int                      `json:"count"`
}

func (s *Service) PreviewApplicationBinding(ctx context.Context, runtimeKind, resourceID string, selector map[string]any) (ApplicationBindingPreview, error) {
	if s.resources == nil {
		return ApplicationBindingPreview{}, invalid("resource service is unavailable")
	}
	item, err := s.resources.Get(ctx, strings.TrimSpace(resourceID))
	if err != nil {
		return ApplicationBindingPreview{}, err
	}
	switch runtimeKind {
	case "virtual_machine":
		if item.Kind != "Host" {
			return ApplicationBindingPreview{}, invalid(fmt.Sprintf("resource %q is not a Host resource", item.Name))
		}
		connection := host.ConnectionInput{}
		setHostInput(&connection, item.Config)
		secret, configured, err := s.resourceSecret(ctx, item.ID)
		if err != nil {
			return ApplicationBindingPreview{}, connectorError(CategoryConfiguration, "read Host credential", false, err)
		}
		if configured {
			var values map[string]any
			if json.Unmarshal(secret, &values) == nil {
				setHostInputIfEmpty(&connection, values)
			}
		}
		keyword, _ := selector["process_keyword"].(string)
		if strings.TrimSpace(keyword) == "" {
			return ApplicationBindingPreview{}, invalid("process_keyword is required")
		}
		output, err := host.Processes(ctx, host.ProcessesInput{ConnectionInput: connection, Keyword: keyword, Limit: host.MaxProcessLimit})
		if err != nil {
			if errors.Is(err, host.ErrInvalidArgument) {
				return ApplicationBindingPreview{}, invalid(err.Error())
			}
			return ApplicationBindingPreview{}, err
		}
		result := ApplicationBindingPreview{Count: output.MatchCount, Items: make([]ApplicationBindingItem, 0, len(output.Processes))}
		for _, process := range output.Processes {
			result.Items = append(result.Items, ApplicationBindingItem{
				ID: fmt.Sprint(process.PID), Name: process.Name,
				Detail: process.CommandLine, Status: process.State,
			})
		}
		return result, nil
	case "containerized":
		if item.Kind != "Docker" {
			return ApplicationBindingPreview{}, invalid(fmt.Sprintf("resource %q is not a Docker resource", item.Name))
		}
		keyword, _ := selector["container_name"].(string)
		terms := bindingTerms(keyword)
		if len(terms) == 0 {
			return ApplicationBindingPreview{}, invalid("container_name is required")
		}
		connection, err := s.dockerConnection(ctx, engine.ContextResource{
			ID: item.ID, ScopeID: item.ScopeID, Kind: item.Kind, Name: item.Name,
			Subtype: item.Subtype, AgentRef: item.AgentRef, Config: item.Config,
		})
		if err != nil {
			return ApplicationBindingPreview{}, err
		}
		output, err := docker.Containers(ctx, docker.ListContainersInput{ConnectionInput: connection, All: true})
		if err != nil {
			return ApplicationBindingPreview{}, err
		}
		result := ApplicationBindingPreview{Items: make([]ApplicationBindingItem, 0)}
		for _, container := range output.Containers {
			name := strings.TrimPrefix(firstString(container.Names), "/")
			searchText := strings.ToLower(strings.Join(append(append([]string{}, container.Names...), container.ID, container.Image), " "))
			if !containsAll(searchText, terms) {
				continue
			}
			result.Count++
			if len(result.Items) < 50 {
				result.Items = append(result.Items, ApplicationBindingItem{ID: container.ID, Name: name, Detail: container.Image, Status: container.Status})
			}
		}
		return result, nil
	default:
		return ApplicationBindingPreview{}, invalid(fmt.Sprintf("runtime kind %q does not support direct binding preview", runtimeKind))
	}
}

func (s *Service) ValidateApplicationBinding(ctx context.Context, runtimeKind, resourceID string, selector map[string]any) error {
	preview, err := s.PreviewApplicationBinding(ctx, runtimeKind, resourceID, selector)
	if err != nil {
		return err
	}
	if preview.Count != 1 {
		return invalid(fmt.Sprintf("application binding must match exactly one target; found %d", preview.Count))
	}
	return nil
}

func bindingTerms(value string) []string {
	parts := strings.Split(value, ",")
	terms := make([]string, 0, len(parts))
	for _, part := range parts {
		if term := strings.ToLower(strings.TrimSpace(part)); term != "" {
			terms = append(terms, term)
		}
	}
	return terms
}

func containsAll(value string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(value, term) {
			return false
		}
	}
	return true
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
