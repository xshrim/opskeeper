package httpapi

import (
	"testing"

	"opskeeper/backend/authorization"
)

func TestUpsertRoleBindingKeepsOneRolePerScope(t *testing.T) {
	bindings := []authorization.RoleBinding{
		{ID: "platform", SubjectType: "user", SubjectID: "user-1", RoleID: "viewer", ScopeID: "team-1"},
		{ID: "other", SubjectType: "user", SubjectID: "user-1", RoleID: "viewer", ScopeID: "team-2"},
	}

	result := upsertRoleBinding(bindings, authorization.RoleBinding{
		ID:          "operator",
		SubjectType: "user",
		SubjectID:   "user-1",
		RoleID:      "operator",
		ScopeID:     "team-1",
	})
	if len(result) != 2 || result[0].ScopeID != "team-2" || result[1].RoleID != "operator" {
		t.Fatalf("upsertRoleBinding() = %#v, want one binding per scope", result)
	}
}
