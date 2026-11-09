package diagnosis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	appdomain "opskeeper/backend/application"
	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

func TestServiceStartRequiresAuthorizedSameScopeTarget(t *testing.T) {
	store := &memoryStore{}
	resources := fakeResources{items: map[string]resource.Resource{
		"target-1": {ID: "target-1", ScopeID: "scope-1"},
		"target-2": {ID: "target-2", ScopeID: "scope-2"},
	}}
	service := NewService(store, resources, nil)
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ScopeIDs: []string{"scope-1"}, ResourceIDs: []string{"target-1"}})
	item, err := service.Start(ctx, StartInput{ScopeID: "scope-1", ActorUserID: "actor-1", Question: "inspect", TargetResourceIDs: []string{"target-1", "target-1"}})
	if err != nil || item.ID == "" || len(store.started.TargetResourceIDs) != 1 {
		t.Fatalf("Start() = %#v, %v", item, err)
	}
	if _, err := service.Start(ctx, StartInput{ScopeID: "scope-1", ActorUserID: "actor-1", Question: "inspect", TargetResourceIDs: []string{"target-2"}}); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("cross-scope target error = %v", err)
	}
}

func TestServiceStartAllowsAuthorizedCrossScopeTarget(t *testing.T) {
	store := &memoryStore{}
	resources := fakeResources{items: map[string]resource.Resource{
		"team-target": {ID: "team-target", ScopeID: "team-scope"},
	}}
	service := NewService(store, resources, nil)
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ScopeIDs: []string{"project-scope", "team-scope"}})
	if _, err := service.Start(ctx, StartInput{ScopeID: "project-scope", ActorUserID: "actor-1", Question: "inspect", TargetResourceIDs: []string{"team-target"}}); err != nil {
		t.Fatalf("Start() with authorized team target error = %v", err)
	}
}

func TestServiceStartUsesResourceUsePermissionForTargets(t *testing.T) {
	store := &memoryStore{}
	resources := fakeResources{items: map[string]resource.Resource{
		"team-target": {ID: "team-target", ScopeID: "team-scope"},
	}}
	service := NewService(store, resources, nil)
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ScopeIDs: []string{"project-scope"}})
	ctx = authorization.WithResourceUseFilter(ctx, authorization.ResourceFilter{ScopeIDs: []string{"team-scope"}})
	if _, err := service.Start(ctx, StartInput{ScopeID: "project-scope", ActorUserID: "actor-1", Question: "inspect", TargetResourceIDs: []string{"team-target"}}); err != nil {
		t.Fatalf("Start() with resource-use-authorized target error = %v", err)
	}
}

func TestServiceStartAllowsProjectApplicationInAuthorizedTeamSession(t *testing.T) {
	store := &memoryStore{}
	resources := fakeResources{items: map[string]resource.Resource{
		"project-runtime": {ID: "project-runtime", ScopeID: "project-scope"},
	}}
	applications := fakeApplications{
		item:      appdomain.Application{ID: "app-1", ProjectScopeID: "project-scope", Status: "active"},
		resources: []string{"project-runtime"},
		context:   appdomain.DiagnosticContext{Application: appdomain.DiagnosticApplication{ID: "app-1"}},
	}
	service := NewService(store, resources, applications)
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ScopeIDs: []string{"team-scope", "project-scope"}})
	ctx = authorization.WithResourceUseFilter(ctx, authorization.ResourceFilter{ScopeIDs: []string{"team-scope", "project-scope"}})
	if _, err := service.Start(ctx, StartInput{ScopeID: "team-scope", ActorUserID: "actor-1", ApplicationIDs: []string{"app-1"}, Question: "inspect"}); err != nil {
		t.Fatalf("Start() with project application in team session error = %v", err)
	}
	if got := store.started.TargetResourceIDs; len(got) != 1 || got[0] != "project-runtime" {
		t.Fatalf("session targets = %v, want project runtime resource", got)
	}
}

func TestServiceAddTargetAllowsAuthorizedCrossScopeResourceButRequiresSessionScope(t *testing.T) {
	store := &memoryStore{session: Session{ID: "session-1", ScopeID: "project-scope", ActorUserID: testActor("actor-1"), Status: StatusQueued}}
	resources := fakeResources{items: map[string]resource.Resource{
		"team-target": {ID: "team-target", ScopeID: "team-scope"},
	}}
	service := NewService(store, resources, nil)
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{SubjectID: "actor-1", ScopeIDs: []string{"project-scope", "team-scope"}})
	if _, err := service.AddTarget(ctx, "session-1", "team-target"); err != nil {
		t.Fatalf("AddTarget() with authorized team target error = %v", err)
	}
	unauthorizedSession := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{SubjectID: "actor-1", ScopeIDs: []string{"team-scope"}})
	if _, err := service.AddTarget(unauthorizedSession, "session-1", "team-target"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("AddTarget() without session-scope permission error = %v, want ErrForbidden", err)
	}
}

func TestServiceAskReopensCompletedSession(t *testing.T) {
	store := &memoryStore{session: Session{ID: "session-1", ScopeID: "scope-1", ActorUserID: testActor("actor-1"), Status: StatusSucceeded}}
	service := NewService(store, fakeResources{}, nil)
	ctx := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{SubjectID: "actor-1", ScopeIDs: []string{"scope-1"}})
	message, err := service.Ask(ctx, "session-1", "continue")
	if err != nil || !store.reopened || message.Role != "user" || message.Content != "continue" {
		t.Fatalf("Ask() = %#v, %v; reopened=%v", message, err, store.reopened)
	}
}

func TestServiceAskRequiresResourceUseForExistingTargets(t *testing.T) {
	store := &memoryStore{
		session: Session{ID: "session-1", ScopeID: "scope-1", Status: StatusSucceeded},
		targets: []Target{{SessionID: "session-1", ResourceID: "resource-1"}},
	}
	service := NewService(store, fakeResources{items: map[string]resource.Resource{
		"resource-1": {ID: "resource-1", ScopeID: "scope-1"},
	}}, nil)
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ScopeIDs: []string{"scope-1"}})
	ctx = authorization.WithResourceUseFilter(ctx, authorization.ResourceFilter{ScopeIDs: []string{"scope-2"}})
	if _, err := service.Ask(ctx, "session-1", "continue"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("Ask() without resource use error = %v, want ErrForbidden", err)
	}
}

func TestServiceAskMessagePersistsComposerMetadata(t *testing.T) {
	store := &memoryStore{session: Session{ID: "session-1", ScopeID: "scope-1", ActorUserID: testActor("actor-1"), Status: StatusSucceeded}}
	service := NewService(store, fakeResources{}, nil)
	ctx := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{SubjectID: "actor-1", ScopeIDs: []string{"scope-1"}})
	input := AppendMessageInput{
		Content:     "investigate",
		SkillID:     "skill-1",
		PersonaID:   "persona-1",
		Links:       []string{"https://example.test/runbook"},
		Attachments: []Attachment{{Name: "app.log", MediaType: "text/plain", Content: "failed to connect"}},
	}
	message, err := service.AskMessage(ctx, "session-1", input)
	if err != nil || message.Content != input.Content || store.appended.SkillID != input.SkillID || store.appended.PersonaID != input.PersonaID || len(store.appended.Links) != 1 || len(store.appended.Attachments) != 1 {
		t.Fatalf("AskMessage() = %#v, %v; stored=%#v", message, err, store.appended)
	}
}

func TestServiceAddTargetReopensCompletedSession(t *testing.T) {
	store := &memoryStore{session: Session{ID: "session-1", ScopeID: "scope-1", ActorUserID: testActor("actor-1"), Status: StatusSucceeded}}
	resources := fakeResources{items: map[string]resource.Resource{
		"host-1": {ID: "host-1", ScopeID: "scope-1"},
	}}
	service := NewService(store, resources, nil)
	ctx := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{SubjectID: "actor-1", ScopeIDs: []string{"scope-1"}})
	if _, err := service.AddTarget(ctx, "session-1", "host-1"); err != nil || !store.reopened {
		t.Fatalf("AddTarget() = %v; reopened=%v", err, store.reopened)
	}
}

func TestServiceKeepsPrivateSessionsWithTheirCreator(t *testing.T) {
	store := &memoryStore{session: Session{ID: "session-1", ScopeID: "scope-1", ActorUserID: testActor("owner-1"), Status: StatusSucceeded}}
	service := NewService(store, fakeResources{}, nil)
	other := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{SubjectID: "other-1", ScopeIDs: []string{"scope-1"}})
	if _, err := service.Get(other, "session-1"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("private session Get() error = %v, want ErrForbidden", err)
	}
	if err := service.Delete(other, "session-1"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("private session Delete() error = %v, want ErrForbidden", err)
	}

	owner := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{SubjectID: "owner-1", ScopeIDs: []string{"scope-1"}})
	updated, err := service.SetScopeShared(owner, "session-1", true)
	if err != nil || !updated.ScopeShared {
		t.Fatalf("SetScopeShared() = %#v, %v", updated, err)
	}
	if _, err := service.Get(other, "session-1"); err != nil {
		t.Fatalf("shared session Get() error = %v", err)
	}
	if _, err := service.Ask(other, "session-1", "continue"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("shared session Ask() error = %v, want ErrForbidden", err)
	}
	if _, err := service.SetScopeShared(other, "session-1", false); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("shared session SetScopeShared() error = %v, want ErrForbidden", err)
	}
}

func TestServiceSharedSnapshotRequiresResourceReadForTargets(t *testing.T) {
	store := &memoryStore{
		session: Session{ID: "session-1", ScopeID: "scope-1", ActorUserID: testActor("owner-1"), ScopeShared: true, Status: StatusSucceeded},
		targets: []Target{{SessionID: "session-1", ResourceID: "resource-1"}},
	}
	service := NewService(store, fakeResources{items: map[string]resource.Resource{
		"resource-1": {ID: "resource-1", ScopeID: "scope-1"},
	}}, nil)
	noRead := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{SubjectID: "reader-1", ScopeIDs: []string{"scope-1"}})
	noRead = authorization.WithResourceUseFilter(noRead, authorization.ResourceFilter{SubjectID: "reader-1"})
	noRead = authorization.WithResourceReadFilter(noRead, authorization.ResourceFilter{SubjectID: "reader-1"})
	if _, err := service.Get(noRead, "session-1"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("shared snapshot without resource read error = %v, want ErrForbidden", err)
	}
	read := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{SubjectID: "reader-1", ScopeIDs: []string{"scope-1"}, ResourceIDs: []string{"resource-1"}})
	read = authorization.WithResourceUseFilter(read, authorization.ResourceFilter{SubjectID: "reader-1"})
	read = authorization.WithResourceReadFilter(read, authorization.ResourceFilter{SubjectID: "reader-1", ResourceIDs: []string{"resource-1"}})
	if _, err := service.Get(read, "session-1"); err != nil {
		t.Fatalf("shared snapshot with resource read error = %v", err)
	}
}

func TestServiceListsSessionsForCurrentActor(t *testing.T) {
	store := &memoryStore{session: Session{ID: "session-1", ScopeID: "scope-1", ActorUserID: testActor("owner-1")}}
	service := NewService(store, fakeResources{}, nil)
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{SubjectID: "owner-1", ScopeIDs: []string{"scope-1"}})
	if _, err := service.List(ctx, "scope-1", 50); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.listActor != "owner-1" {
		t.Fatalf("List actor = %q, want owner-1", store.listActor)
	}
}

func TestServiceStartAddsApplicationContextResources(t *testing.T) {
	store := &memoryStore{}
	resources := fakeResources{items: map[string]resource.Resource{
		"runtime-1":  {ID: "runtime-1", ScopeID: "scope-1"},
		"database-1": {ID: "database-1", ScopeID: "scope-1"},
		"explicit-1": {ID: "explicit-1", ScopeID: "scope-1"},
	}}
	applications := fakeApplications{
		item:      appdomain.Application{ID: "app-1", ProjectScopeID: "scope-1", Status: "active"},
		resources: []string{"runtime-1", "database-1"},
		context: appdomain.DiagnosticContext{
			Application: appdomain.DiagnosticApplication{ID: "app-1", Name: "Orders", Code: "orders"},
			Resources:   []appdomain.DiagnosticResource{{ID: "database-1", Relation: "association", Binding: map[string]any{"database": "orders"}}},
		},
	}
	service := NewService(store, resources, applications)
	ctx := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{ScopeIDs: []string{"scope-1"}})
	if _, err := service.Start(ctx, StartInput{ScopeID: "scope-1", ActorUserID: "actor-1", ApplicationIDs: []string{"app-1", "app-2"}, Question: "inspect", TargetResourceIDs: []string{"explicit-1"}}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	want := []string{"runtime-1", "database-1", "explicit-1"}
	if len(store.started.TargetResourceIDs) != len(want) {
		t.Fatalf("context resources = %v, want %v", store.started.TargetResourceIDs, want)
	}
	if len(store.started.ApplicationIDs) != 2 || store.started.ApplicationID != "app-1" {
		t.Fatalf("stored application ids = %v / %q", store.started.ApplicationIDs, store.started.ApplicationID)
	}
	for index, id := range want {
		if store.started.TargetResourceIDs[index] != id {
			t.Fatalf("context resources = %v, want %v", store.started.TargetResourceIDs, want)
		}
	}
	var snapshot struct {
		Applications []appdomain.DiagnosticContext `json:"applications"`
	}
	if err := json.Unmarshal(store.started.ApplicationContext, &snapshot); err != nil {
		t.Fatalf("application context snapshot is not JSON: %v", err)
	}
	if len(snapshot.Applications) != 2 || snapshot.Applications[0].Application.Name != "Orders" || snapshot.Applications[1].Application.ID != "app-2" || len(snapshot.Applications[0].Resources) != 1 || snapshot.Applications[0].Resources[0].Binding["database"] != "orders" {
		t.Fatalf("application context snapshot = %#v", snapshot)
	}
}

type fakeResources struct{ items map[string]resource.Resource }

func (f fakeResources) Get(_ context.Context, id string) (resource.Resource, error) {
	item, ok := f.items[id]
	if !ok {
		return resource.Resource{}, resource.ErrNotFound
	}
	return item, nil
}

type fakeApplications struct {
	item      appdomain.Application
	resources []string
	context   appdomain.DiagnosticContext
}

func (f fakeApplications) Get(context.Context, string) (appdomain.Application, error) {
	return f.item, nil
}

func (f fakeApplications) ContextResourceIDs(context.Context, string, string) ([]string, error) {
	return append([]string(nil), f.resources...), nil
}
func (f fakeApplications) DiagnosticContext(_ context.Context, applicationID, _ string) (appdomain.DiagnosticContext, error) {
	item := f.context
	item.Application.ID = applicationID
	return item, nil
}

type memoryStore struct {
	session   Session
	started   StartInput
	appended  AppendMessageInput
	reopened  bool
	targets   []Target
	listActor string
}

func testActor(id string) *string { return &id }

func (m *memoryStore) Start(_ context.Context, input StartInput) (Session, error) {
	m.started = input
	m.session = Session{ID: "session-1", ScopeID: input.ScopeID, ActorUserID: testActor(input.ActorUserID), ScopeShared: input.ScopeShared, Status: StatusQueued}
	return m.session, nil
}
func (m *memoryStore) Get(context.Context, string) (Session, error) { return m.session, nil }
func (m *memoryStore) List(_ context.Context, _ string, actorID string, _ int) ([]Session, error) {
	m.listActor = actorID
	return []Session{m.session}, nil
}
func (m *memoryStore) SetScopeShared(_ context.Context, _ string, shared bool) (Session, error) {
	m.session.ScopeShared = shared
	return m.session, nil
}
func (*memoryStore) Delete(context.Context, string) error                      { return nil }
func (m *memoryStore) Targets(context.Context, string) ([]Target, error)       { return m.targets, nil }
func (*memoryStore) AddTarget(context.Context, string, string) (Target, error) { return Target{}, nil }
func (*memoryStore) RemoveTarget(context.Context, string, string) error        { return nil }
func (*memoryStore) Messages(context.Context, string, int) ([]Message, error)  { return nil, nil }
func (m *memoryStore) AppendMessage(_ context.Context, id string, input AppendMessageInput) (Message, error) {
	m.appended = input
	return Message{ID: "message-1", SessionID: id, Role: input.Role, Content: input.Content, CreatedAt: time.Now()}, nil
}
func (*memoryStore) CreatePlan(context.Context, string, string, []PlanStep) (Plan, error) {
	return Plan{}, nil
}
func (*memoryStore) Plan(context.Context, string) (Plan, error) { return Plan{}, ErrNotFound }
func (*memoryStore) UpdateStep(context.Context, string, string, string) (PlanStep, error) {
	return PlanStep{}, nil
}
func (*memoryStore) AppendEvent(context.Context, string, CreateEventInput) (Event, error) {
	return Event{}, nil
}
func (*memoryStore) EventsAfter(context.Context, string, int64, int) ([]Event, error) {
	return nil, nil
}
func (*memoryStore) SaveEvidence(context.Context, string, CreateEvidenceInput) (Evidence, error) {
	return Evidence{}, nil
}
func (*memoryStore) Evidence(context.Context, string) ([]Evidence, error) { return nil, nil }
func (*memoryStore) CreateRun(context.Context, string, string) (Run, error) {
	return Run{ID: "run-1", Status: "running"}, nil
}
func (*memoryStore) FinishRun(context.Context, string, string) (Run, error) { return Run{}, nil }
func (*memoryStore) Runs(context.Context, string) ([]Run, error)            { return nil, nil }
func (*memoryStore) SaveCausalChain(context.Context, CausalChain) (CausalChain, error) {
	return CausalChain{}, nil
}
func (*memoryStore) CausalChains(context.Context, string) ([]CausalChain, error) { return nil, nil }
func (*memoryStore) SaveHypothesis(context.Context, Hypothesis) (Hypothesis, error) {
	return Hypothesis{}, nil
}
func (*memoryStore) Hypotheses(context.Context, string) ([]Hypothesis, error) { return nil, nil }
func (*memoryStore) SaveReport(context.Context, Report) (Report, error)       { return Report{}, nil }
func (*memoryStore) Report(context.Context, string) (Report, error)           { return Report{}, ErrNotFound }
func (m *memoryStore) SetStatus(context.Context, string, Status) (Session, error) {
	return m.session, nil
}
func (m *memoryStore) ClaimRun(context.Context, string) (Session, bool, error) {
	m.session.Status = StatusPlanning
	return m.session, true, nil
}
func (m *memoryStore) Reopen(context.Context, string) (Session, error) {
	m.reopened = true
	m.session.Status = StatusQueued
	return m.session, nil
}
func (*memoryStore) Finish(context.Context, string, Status, string, string) (Session, error) {
	return Session{}, nil
}
