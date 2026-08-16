package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"opskeeper/backend/organization"
	"opskeeper/backend/resource"
)

const scanTimeout = 5 * time.Minute

type Service struct {
	store       Store
	resources   ResourceReader
	importer    ResourceImporter
	projects    ProjectManager
	credentials CredentialReader
	scanner     Scanner
}

func NewService(store Store, resources ResourceReader, importer ResourceImporter, projects ProjectManager, credentials CredentialReader, scanner Scanner) *Service {
	return &Service{store: store, resources: resources, importer: importer, projects: projects, credentials: credentials, scanner: scanner}
}

func (s *Service) Start(ctx context.Context, actorID, clusterID string) (Run, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return Run{}, fmt.Errorf("%w: cluster_resource_id is required", ErrInvalid)
	}
	cluster, err := s.resources.Get(ctx, clusterID)
	if err != nil {
		return Run{}, err
	}
	if cluster.Kind != "Kubernetes" {
		return Run{}, fmt.Errorf("%w: resource is not Kubernetes", ErrInvalid)
	}
	if cluster.CredentialID == nil || strings.TrimSpace(*cluster.CredentialID) == "" {
		return Run{}, fmt.Errorf("%w: Kubernetes requires a kubeconfig credential", ErrInvalid)
	}
	run, err := s.store.CreateRun(ctx, cluster.ID, actorID)
	if err != nil {
		return Run{}, err
	}
	go s.execute(context.WithoutCancel(ctx), run.ID, cluster)
	return run, nil
}

func (s *Service) execute(ctx context.Context, runID string, cluster resource.Resource) {
	if err := s.store.SetRunning(ctx, runID); err != nil {
		_ = s.store.FailRun(ctx, runID, err)
		return
	}
	secret, err := s.credentials.RevealLinked(ctx, *cluster.CredentialID)
	if err != nil {
		_ = s.store.FailRun(ctx, runID, fmt.Errorf("read cluster credential: %w", err))
		return
	}
	kubeconfig, err := kubeconfigFromSecret(secret)
	if err != nil {
		_ = s.store.FailRun(ctx, runID, err)
		return
	}
	scanCtx, cancel := context.WithTimeout(ctx, scanTimeout)
	items, err := s.scanner.Scan(scanCtx, cluster, kubeconfig)
	cancel()
	if err != nil {
		_ = s.store.FailRun(ctx, runID, err)
		return
	}
	if err := validateScannedItems(items); err != nil {
		_ = s.store.FailRun(ctx, runID, err)
		return
	}
	uids := make([]string, 0, len(items))
	for _, item := range items {
		uids = append(uids, item.ExternalUID)
	}
	if err := s.store.ReplaceItems(ctx, runID, items); err != nil {
		_ = s.store.FailRun(ctx, runID, err)
		return
	}
	if err := s.store.MarkMissing(ctx, cluster.ID, uids); err != nil {
		_ = s.store.FailRun(ctx, runID, err)
		return
	}
	if err := s.store.CompleteRun(ctx, runID); err != nil {
		_ = s.store.FailRun(ctx, runID, err)
	}
}

func (s *Service) Get(ctx context.Context, runID string) (Run, error) {
	if strings.TrimSpace(runID) == "" {
		return Run{}, fmt.Errorf("%w: discovery_id is required", ErrInvalid)
	}
	return s.store.GetRun(ctx, runID)
}

func (s *Service) List(ctx context.Context, clusterID string) ([]Run, error) {
	if strings.TrimSpace(clusterID) == "" {
		return nil, fmt.Errorf("%w: cluster_resource_id is required", ErrInvalid)
	}
	cluster, err := s.resources.Get(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if cluster.Kind != "Kubernetes" {
		return nil, fmt.Errorf("%w: resource is not Kubernetes", ErrInvalid)
	}
	return s.store.ListRuns(ctx, clusterID)
}

func (s *Service) Items(ctx context.Context, runID string) ([]Item, error) {
	if _, err := s.store.GetRun(ctx, runID); err != nil {
		return nil, err
	}
	return s.store.ListItems(ctx, runID)
}

func (s *Service) Import(ctx context.Context, actorID, runID string, input ImportInput) (ImportResult, error) {
	run, err := s.store.GetRun(ctx, runID)
	if err != nil {
		return ImportResult{}, err
	}
	if run.Status != RunSucceeded {
		return ImportResult{}, fmt.Errorf("%w: discovery must succeed before import", ErrConflict)
	}
	cluster, err := s.resources.Get(ctx, run.ClusterResourceID)
	if err != nil {
		return ImportResult{}, err
	}
	items, err := s.store.ListItems(ctx, runID)
	if err != nil {
		return ImportResult{}, err
	}
	selected := stringSet(input.ItemIDs)
	result := ImportResult{Imported: make([]Item, 0), Ignored: make([]Item, 0)}
	namespaceProjects := make(map[string]organization.Project)

	for _, item := range items {
		if item.Kind != "Project" {
			continue
		}
		mapping, exists := input.ProjectMappings[item.Namespace]
		if !exists {
			continue
		}
		if mapping.Ignore {
			if err := s.store.MarkIgnored(ctx, item.ID); err != nil {
				return ImportResult{}, err
			}
			item.Status = ItemIgnored
			result.Ignored = append(result.Ignored, item)
			continue
		}
		project, err := s.mapProject(ctx, cluster, item, mapping)
		if err != nil {
			return ImportResult{}, err
		}
		if err := s.store.MarkProjectMapped(ctx, item.ID, project.ID, runID); err != nil {
			return ImportResult{}, err
		}
		item.Status = ItemImported
		item.ImportedProjectID = &project.ID
		namespaceProjects[item.Namespace] = project
		result.Imported = append(result.Imported, item)
	}

	for _, item := range items {
		if item.Kind != "Application" {
			continue
		}
		if _, ok := selected[item.ID]; !ok {
			continue
		}
		mapping, mapped := input.ProjectMappings[item.Namespace]
		if mapped && mapping.Ignore {
			if err := s.store.MarkIgnored(ctx, item.ID); err != nil {
				return ImportResult{}, err
			}
			item.Status = ItemIgnored
			result.Ignored = append(result.Ignored, item)
			continue
		}
		project, ok := namespaceProjects[item.Namespace]
		if !ok {
			return ImportResult{}, fmt.Errorf("%w: namespace %q must be mapped to a project before importing applications", ErrInvalid, item.Namespace)
		}
		config := cloneMap(item.Payload)
		config["cluster_resource_id"] = cluster.ID
		config["namespace"] = item.Namespace
		config["resource_version"] = item.ResourceVersion
		imported, err := s.importer.Import(ctx, resource.ImportedInput{
			ScopeID:          project.Scope.ID,
			Kind:             "Application",
			Name:             item.Name,
			ExternalUID:      item.ExternalUID,
			SourceResourceID: cluster.ID,
			Labels:           item.Labels,
			Config:           config,
			Status:           resource.StatusActive,
		})
		if err != nil {
			return ImportResult{}, err
		}
		if err := s.store.MarkImported(ctx, item.ID, imported.ID, runID); err != nil {
			return ImportResult{}, err
		}
		item.Status = ItemImported
		item.ImportedResourceID = &imported.ID
		result.Imported = append(result.Imported, item)
	}
	result.Run, err = s.store.GetRun(ctx, runID)
	return result, err
}

func (s *Service) mapProject(ctx context.Context, cluster resource.Resource, item Item, mapping ProjectMapping) (organization.Project, error) {
	config := cloneMap(item.Payload)
	config["cluster_resource_id"] = cluster.ID
	config["namespace"] = item.Namespace
	config["resource_version"] = item.ResourceVersion
	if strings.TrimSpace(mapping.ProjectID) != "" {
		project, err := s.projects.GetProject(ctx, mapping.ProjectID)
		if err != nil {
			return organization.Project{}, err
		}
		if err := s.store.ValidateImportTarget(ctx, cluster.ScopeID, project.Scope.ID); err != nil {
			return organization.Project{}, fmt.Errorf("%w: project is outside the Kubernetes cluster scope", ErrInvalid)
		}
		return s.projects.BindProjectSource(ctx, project.ID, organization.ProjectSourceInput{
			SourceResourceID: cluster.ID,
			ExternalUID:      item.ExternalUID,
			SourceConfig:     config,
		})
	}
	team, err := s.projects.GetTeam(ctx, mapping.TeamID)
	if err != nil {
		return organization.Project{}, err
	}
	if err := s.store.ValidateProjectParent(ctx, cluster.ScopeID, team.Scope.ID); err != nil {
		return organization.Project{}, fmt.Errorf("%w: target team is outside the Kubernetes cluster scope", ErrInvalid)
	}
	name := strings.TrimSpace(mapping.Name)
	if name == "" {
		name = item.Name
	}
	code := strings.TrimSpace(mapping.Code)
	if code == "" {
		code = namespaceCode(item.Namespace)
	}
	clusterID := cluster.ID
	return s.projects.CreateProject(ctx, organization.CreateProjectInput{
		TeamID:           team.ID,
		Name:             name,
		Code:             code,
		Icon:             "project",
		Labels:           item.Labels,
		Source:           "kubernetes",
		SourceResourceID: &clusterID,
		ExternalUID:      item.ExternalUID,
		SourceConfig:     config,
	})
}

func kubeconfigFromSecret(secret []byte) (string, error) {
	value := strings.TrimSpace(string(secret))
	if value == "" {
		return "", fmt.Errorf("%w: kubeconfig credential is empty", ErrInvalid)
	}
	var fields map[string]string
	if json.Unmarshal(secret, &fields) == nil {
		value = strings.TrimSpace(fields["kubeconfig"])
	}
	if value == "" {
		return "", fmt.Errorf("%w: credential does not contain kubeconfig", ErrInvalid)
	}
	return value, nil
}

func validateScannedItems(items []ScannedItem) error {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.ExternalUID) == "" {
			return fmt.Errorf("%w: discovered item requires name and UID", ErrInvalid)
		}
		switch item.Kind {
		case "Project", "Application":
		default:
			return fmt.Errorf("%w: unsupported discovered kind %q", ErrInvalid, item.Kind)
		}
		key := item.Kind + "\x00" + item.ExternalUID
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%w: duplicate discovered identity", ErrInvalid)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func namespaceCode(namespace string) string {
	value := strings.ToLower(strings.TrimSpace(namespace))
	value = strings.ReplaceAll(value, "_", "-")
	if value == "" {
		return "kubernetes-project"
	}
	return value
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value)+2)
	for key, item := range value {
		result[key] = item
	}
	return result
}
