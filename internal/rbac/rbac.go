package rbac

type Role string

const (
	RoleOwner      Role = "owner"
	RoleAdmin      Role = "admin"
	RoleMaintainer Role = "maintainer"
	RoleDeveloper  Role = "developer"
	RoleViewer     Role = "viewer"
)

type Permission string

const (
	PermOrgRead          Permission = "org.read"
	PermOrgUpdate        Permission = "org.update"
	PermOrgManageMembers Permission = "org.manage_members"
	PermProjectCreate    Permission = "project.create"
	PermProjectRead      Permission = "project.read"
	PermProjectUpdate    Permission = "project.update"
	PermProjectDelete    Permission = "project.delete"
)

var rolePermissions = map[Role]map[Permission]struct{}{
	RoleOwner: {
		PermOrgRead: {}, PermOrgUpdate: {}, PermOrgManageMembers: {},
		PermProjectCreate: {}, PermProjectRead: {}, PermProjectUpdate: {}, PermProjectDelete: {},
	},
	RoleAdmin: {
		PermOrgRead: {}, PermOrgUpdate: {}, PermOrgManageMembers: {},
		PermProjectCreate: {}, PermProjectRead: {}, PermProjectUpdate: {}, PermProjectDelete: {},
	},
	RoleMaintainer: {
		PermOrgRead: {},
		PermProjectCreate: {}, PermProjectRead: {}, PermProjectUpdate: {},
	},
	RoleDeveloper: {
		PermOrgRead: {},
		PermProjectRead: {}, PermProjectUpdate: {},
	},
	RoleViewer: {
		PermOrgRead: {},
		PermProjectRead: {},
	},
}

func Can(role Role, perm Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	_, allowed := perms[perm]
	return allowed
}

func ParseRole(raw string) (Role, bool) {
	role := Role(raw)
	_, ok := rolePermissions[role]
	return role, ok
}
