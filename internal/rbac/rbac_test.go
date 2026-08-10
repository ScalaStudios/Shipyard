package rbac

import "testing"

func TestOwnerCanManage(t *testing.T) {
	t.Parallel()
	if !Can(RoleOwner, PermOrgManageMembers) {
		t.Fatal("owner should manage members")
	}
	if Can(RoleViewer, PermProjectCreate) {
		t.Fatal("viewer should not create projects")
	}
	if !Can(RoleDeveloper, PermProjectUpdate) {
		t.Fatal("developer should update projects")
	}
}
