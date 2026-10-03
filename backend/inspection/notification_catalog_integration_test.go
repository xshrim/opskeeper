//go:build integration

package inspection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"opskeeper/backend/authorization"
	"opskeeper/backend/migrations"
	"opskeeper/backend/notification"
	"opskeeper/backend/secret"
)

func TestNotificationCatalogScopeRoutesAndTemplateLifecycle(t *testing.T) {
	ctx := context.Background()
	pool := inspectionIntegrationPool(t)
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var scopeID, policyID, teamScopeID, firstProjectScopeID, secondProjectScopeID, firstProjectPolicyID, secondProjectPolicyID string
	if err := pool.QueryRow(ctx, `INSERT INTO scopes(scope_type) VALUES('platform') RETURNING id::text`).Scan(&scopeID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO inspection_policies(scope_id,name,cron,timezone,timeout_seconds) VALUES($1::uuid,'catalog-'||gen_random_uuid()::text,'* * * * *','UTC',10) RETURNING id::text`, scopeID).Scan(&policyID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO scopes(scope_type,parent_scope_id) VALUES('team',$1::uuid) RETURNING id::text`, scopeID).Scan(&teamScopeID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO scopes(scope_type,parent_scope_id) VALUES('project',$1::uuid) RETURNING id::text`, teamScopeID).Scan(&firstProjectScopeID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO scopes(scope_type,parent_scope_id) VALUES('project',$1::uuid) RETURNING id::text`, teamScopeID).Scan(&secondProjectScopeID); err != nil {
		t.Fatal(err)
	}
	for index, projectScopeID := range []string{firstProjectScopeID, secondProjectScopeID} {
		policyTarget := []*string{&firstProjectPolicyID, &secondProjectPolicyID}[index]
		if err := pool.QueryRow(ctx, `INSERT INTO inspection_policies(scope_id,name,cron,timezone,timeout_seconds) VALUES($1::uuid,'shared-'||gen_random_uuid()::text,'* * * * *','UTC',10) RETURNING id::text`, projectScopeID).Scan(policyTarget); err != nil {
			t.Fatal(err)
		}
	}
	cipher, err := secret.NewLocalEncryptor(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewStore(pool), nil).WithNotificationSecurity(cipher, notification.DefaultProviderRegistry())
	scopeContext := authorization.WithScopeFilter(ctx, authorization.ScopeFilter{ScopeIDs: []string{scopeID}})
	channel, err := service.CreateConfiguredChannel(scopeContext, NotificationChannel{ScopeID: scopeID, Name: "Catalog webhook", Kind: "webhook", ShareWithChildren: true}, map[string]string{"url": "https://notify.example.test", "signing_secret": "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	template, err := service.CreateNotificationTemplate(scopeContext, scopeID, "Findings", "", true, notification.TemplateDraft{
		Format: "markdown", TitleTemplate: "{{.severity}}", BodyTemplate: "{{.finding_summary}}",
		Variables: []notification.TemplateVariable{{Name: "severity", Required: true}, {Name: "finding_summary", Required: true}},
	})
	if err != nil || len(template.Versions) != 1 || template.Versions[0].Status != "draft" {
		t.Fatalf("CreateNotificationTemplate() = %+v, %v", template, err)
	}
	draft := template.Versions[0]
	preview, err := service.PreviewNotificationTemplate(scopeContext, scopeID, template.ID, draft.ID, map[string]string{"severity": "warning", "finding_summary": "disk is full"})
	if err != nil || preview.Title != "warning" || preview.Body != "disk is full" {
		t.Fatalf("PreviewNotificationTemplate() = %+v, %v", preview, err)
	}
	if _, err := service.PreviewNotificationTemplate(scopeContext, scopeID, template.ID, draft.ID, map[string]string{"secret": "not allowed"}); err == nil {
		t.Fatal("preview accepted undeclared input")
	}
	published, err := service.PublishNotificationTemplate(scopeContext, scopeID, template.ID, draft.ID)
	if err != nil || published.Status != "published" {
		t.Fatalf("PublishNotificationTemplate() = %+v, %v", published, err)
	}
	draft2, err := service.CreateNotificationTemplateVersion(scopeContext, scopeID, template.ID, "", notification.TemplateDraft{
		Format: "markdown", TitleTemplate: "{{.severity}} changed", BodyTemplate: "{{.finding_summary}}",
		Variables: []notification.TemplateVariable{{Name: "severity", Required: true}, {Name: "finding_summary", Required: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	published2, err := service.PublishNotificationTemplate(scopeContext, scopeID, template.ID, draft2.ID)
	if err != nil || published2.Status != "published" {
		t.Fatalf("PublishNotificationTemplate(second version) = %+v, %v", published2, err)
	}
	rule, err := service.CreateNotificationRule(scopeContext, scopeID, "", NotificationRuleRecord{
		Rule:              notification.Rule{Name: "Critical finding", EventTypes: []notification.EventType{notification.FindingOpened, "notification.test"}, MinimumSeverity: "critical", Cooldown: time.Minute, Aggregation: time.Minute, MaxBatchSize: 3},
		ShareWithChildren: true,
		Routes:            []NotificationRoute{{ChannelID: channel.ID, TemplateVersionID: published.ID}},
	})
	if err != nil || len(rule.Routes) != 1 {
		t.Fatalf("CreateNotificationRule() = %+v, %v", rule, err)
	}
	updatedRule := rule
	updatedRule.Routes = []NotificationRoute{{ChannelID: channel.ID, TemplateVersionID: published2.ID}}
	if _, err := service.UpdateNotificationRule(scopeContext, scopeID, rule.ID, updatedRule); err != nil {
		t.Fatalf("UpdateNotificationRule(new route) = %v", err)
	}
	updatedRule.Routes = rule.Routes
	if _, err := service.UpdateNotificationRule(scopeContext, scopeID, rule.ID, updatedRule); err != nil {
		t.Fatalf("UpdateNotificationRule(restored historical route) = %v", err)
	}
	if _, err := service.UpdateNotificationRule(scopeContext, scopeID, rule.ID, NotificationRuleRecord{
		Rule: updatedRule.Rule, Routes: []NotificationRoute{{ChannelID: "00000000-0000-0000-0000-000000000000", TemplateVersionID: published.ID}},
	}); err == nil {
		t.Fatal("UpdateNotificationRule accepted a route outside the rule Scope")
	}
	activeRules, err := service.ListNotificationRules(scopeContext, scopeID)
	if err != nil || len(activeRules) != 1 || len(activeRules[0].Routes) != 1 || activeRules[0].Routes[0].TemplateVersionID != published.ID {
		t.Fatalf("active routes after rollback = %+v, %v", activeRules, err)
	}
	var testPayload []byte
	service.tester = notificationSenderFunc(func(_ context.Context, _ NotificationChannel, _ []byte, event WebhookEvent) (int, string, error) {
		if event.Type != "notification.test" {
			t.Fatalf("rule test event type = %q", event.Type)
		}
		testPayload = append(testPayload[:0], event.Data...)
		return 204, "", nil
	})
	routesTested, err := service.TestNotificationRule(scopeContext, scopeID, rule.ID)
	if err != nil || routesTested != 1 || !bytes.Contains(testPayload, []byte("Test notification route")) {
		t.Fatalf("TestNotificationRule() = %d, payload %s, %v", routesTested, testPayload, err)
	}
	var routeHistory, retiredRoutes int
	if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE retired_at IS NOT NULL) FROM notification_rule_routes WHERE rule_id=$1::uuid`, rule.ID).Scan(&routeHistory, &retiredRoutes); err != nil || routeHistory != 3 || retiredRoutes != 2 {
		t.Fatalf("route history = %d rows, %d retired, %v; want 3 rows and 2 retired", routeHistory, retiredRoutes, err)
	}
	rules, err := service.SetPolicyNotificationRules(scopeContext, scopeID, policyID, []string{rule.ID})
	if err != nil || len(rules) != 1 || rules[0] != rule.ID {
		t.Fatalf("SetPolicyNotificationRules() = %v, %v", rules, err)
	}
	rules, err = service.ListPolicyNotificationRules(scopeContext, scopeID, policyID)
	if err != nil || len(rules) != 1 || rules[0] != rule.ID {
		t.Fatalf("ListPolicyNotificationRules() = %v, %v", rules, err)
	}
	projectContextA := authorization.WithScopeFilter(ctx, authorization.ScopeFilter{ScopeIDs: []string{firstProjectScopeID}})
	projectContextB := authorization.WithScopeFilter(ctx, authorization.ScopeFilter{ScopeIDs: []string{secondProjectScopeID}})
	projectChannels, err := service.ListChannels(projectContextA, firstProjectScopeID)
	if err != nil || len(projectChannels) != 1 || !projectChannels[0].Inherited || projectChannels[0].ScopeID != scopeID {
		t.Fatalf("project inherited channels = %+v, %v", projectChannels, err)
	}
	projectTemplates, err := service.ListNotificationTemplates(projectContextA, firstProjectScopeID)
	if err != nil || len(projectTemplates) != 1 || !projectTemplates[0].Inherited || projectTemplates[0].ScopeID != scopeID || len(projectTemplates[0].Versions) == 0 || projectTemplates[0].Versions[0].ScopeID != scopeID {
		t.Fatalf("project inherited templates = %+v, %v", projectTemplates, err)
	}
	projectRules, err := service.ListNotificationRules(projectContextA, firstProjectScopeID)
	if err != nil || len(projectRules) != 1 || !projectRules[0].Inherited || projectRules[0].ScopeID != scopeID {
		t.Fatalf("project inherited rules = %+v, %v", projectRules, err)
	}
	if _, err := service.SetPolicyNotificationRules(projectContextA, firstProjectScopeID, firstProjectPolicyID, []string{rule.ID}); err != nil {
		t.Fatalf("bind shared rule to project A policy: %v", err)
	}
	if _, err := service.SetPolicyNotificationRules(projectContextB, secondProjectScopeID, secondProjectPolicyID, []string{rule.ID}); err != nil {
		t.Fatalf("bind shared rule to project B policy: %v", err)
	}
	privateChannel, err := service.CreateConfiguredChannel(scopeContext, NotificationChannel{ScopeID: scopeID, Name: "Private route", Kind: "webhook"}, map[string]string{"url": "https://private.example.test", "signing_secret": "private-secret"})
	if err != nil {
		t.Fatal(err)
	}
	privateTemplate, err := service.CreateNotificationTemplate(scopeContext, scopeID, "Private template", "", false, notification.TemplateDraft{
		Format: "markdown", TitleTemplate: "{{.severity}}", BodyTemplate: "{{.finding_summary}}",
		Variables: []notification.TemplateVariable{{Name: "severity", Required: true}, {Name: "finding_summary", Required: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	privateVersion, err := service.PublishNotificationTemplate(scopeContext, scopeID, privateTemplate.ID, privateTemplate.Versions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateNotificationRule(scopeContext, scopeID, rule.ID, NotificationRuleRecord{
		Rule: rule.Rule, Routes: []NotificationRoute{{ChannelID: privateChannel.ID, TemplateVersionID: privateVersion.ID}}, ShareWithChildren: true, ShareWithChildrenSet: true,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("replace a shared route with private channel and template = %v, want conflict", err)
	}
	if err := service.SetNotificationTemplateSharing(scopeContext, scopeID, template.ID, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("revoke template sharing while child policies use it = %v, want conflict", err)
	}
	if _, err := service.UpdateConfiguredChannel(scopeContext, scopeID, channel.ID, NotificationChannel{ShareWithChildren: false, ShareWithChildrenSet: true}, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("revoke channel sharing while child policies use it = %v, want conflict", err)
	}
	if _, err := service.UpdateNotificationRule(scopeContext, scopeID, rule.ID, NotificationRuleRecord{
		Rule: rule.Rule, Routes: rule.Routes, ShareWithChildrenSet: true,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("revoke rule sharing while child policies use it = %v, want conflict", err)
	}
	if routesTested, err := service.TestNotificationRule(projectContextA, firstProjectScopeID, rule.ID); err != nil || routesTested != 1 {
		t.Fatalf("inherited TestNotificationRule() = %d, %v", routesTested, err)
	}
	catalogStore := &store{pool: pool}
	for _, item := range []struct{ scopeID, policyID, key string }{
		{firstProjectScopeID, firstProjectPolicyID, "project-a-test"},
		{secondProjectScopeID, secondProjectPolicyID, "project-b-test"},
	} {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := catalogStore.recordNotificationEvent(ctx, tx, notificationEventInput{ScopeID: item.scopeID, PolicyID: item.policyID, Type: "notification.test", Key: item.key, Payload: map[string]any{"event_type": "notification.test"}}); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("enqueue inherited notification event: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE notification_deliveries SET available_at=now()`); err != nil {
		t.Fatal(err)
	}
	jobA, claimed, err := catalogStore.ClaimNotificationDelivery(ctx, "scope-test-a", time.Minute)
	if err != nil || !claimed || len(jobA.Items) != 1 || (jobA.ScopeID != firstProjectScopeID && jobA.ScopeID != secondProjectScopeID) {
		t.Fatalf("first inherited delivery claim = %+v, %t, %v", jobA, claimed, err)
	}
	jobB, claimed, err := catalogStore.ClaimNotificationDelivery(ctx, "scope-test-b", time.Minute)
	if err != nil || !claimed || len(jobB.Items) != 1 || jobB.ScopeID == jobA.ScopeID {
		t.Fatalf("second inherited delivery claim = %+v, %t, %v; first Scope %s", jobB, claimed, err, jobA.ScopeID)
	}
	if err := service.SetNotificationTemplateSharing(scopeContext, scopeID, template.ID, false); err == nil {
		t.Fatal("template sharing was revoked while child policies use the rule")
	}
	if _, err := service.UpdateNotificationRule(projectContextA, firstProjectScopeID, rule.ID, rule); err == nil {
		t.Fatal("child Scope updated a parent notification rule")
	}
	if _, err := service.UpdateConfiguredChannel(projectContextA, firstProjectScopeID, channel.ID, NotificationChannel{Name: "forbidden"}, nil); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("child Scope channel update error = %v, want forbidden", err)
	}
	if _, err := service.SetPolicyNotificationRules(scopeContext, scopeID, policyID, []string{"00000000-0000-0000-0000-000000000000"}); err == nil {
		t.Fatal("policy accepted a rule from another or missing Scope")
	}
	items, err := service.ListNotificationDeliveries(scopeContext, scopeID, "", "", 100)
	if err != nil || len(items) != 0 {
		t.Fatalf("ListNotificationDeliveries() = %v, %v", items, err)
	}
	if _, err := service.ListNotificationTemplates(ctx, scopeID); err == nil {
		t.Fatal("unfiltered context read a notification Scope")
	}
	public, err := json.Marshal(channel)
	if err != nil || string(public) == "" || containsAny(string(public), "test-secret", "notify.example.test") {
		t.Fatalf("public channel response exposed provider secret: %s (%v)", public, err)
	}
}

func containsAny(value string, secrets ...string) bool {
	for _, secret := range secrets {
		if secret != "" && strings.Contains(value, secret) {
			return true
		}
	}
	return false
}
