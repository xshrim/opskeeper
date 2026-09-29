package authorization

import "testing"

func TestRoleDominatesByPermissionSet(t *testing.T) {
	viewer := RoleDefinition{ScopeType: "team", Permissions: []Permission{"organization:read", "resource:read"}}
	operator := RoleDefinition{ScopeType: "team", Permissions: []Permission{"organization:read", "resource:read", "resource:use"}}
	projectViewer := RoleDefinition{ScopeType: "project", Permissions: viewer.Permissions}

	if !roleDominates(operator, viewer) {
		t.Fatal("operator should dominate viewer when it contains all viewer permissions")
	}
	if roleDominates(viewer, operator) {
		t.Fatal("viewer should not dominate operator")
	}
	if roleDominates(operator, projectViewer) {
		t.Fatal("roles at different scope levels should not dominate each other")
	}
}

func TestResourceRoleDominatesByPermissionSet(t *testing.T) {
	viewer := ResourceRoleDefinition{Permissions: []Permission{"resource:read"}}
	operator := ResourceRoleDefinition{Permissions: []Permission{"resource:read", "resource:use"}}

	if !resourceRoleDominates(operator, viewer) {
		t.Fatal("resource operator should dominate resource viewer")
	}
	if resourceRoleDominates(viewer, operator) {
		t.Fatal("resource viewer should not dominate resource operator")
	}
}
