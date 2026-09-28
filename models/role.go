package models

import (
	"slices"

	"github.com/google/uuid"
)

type RoleBinding struct {
	Id          uuid.UUID
	TenantId    uuid.UUID
	OrgId       uuid.UUID
	UserId      *UserId
	ApiKeyId    *uuid.UUID
	Role        Role
	Permissions []Permission
}

// Equivalent reports whether two bindings grant the same role.
func (b RoleBinding) Equivalent(other RoleBinding) bool {
	return b.Role == other.Role
}

// AppliesTo reports whether the binding grants its role when acting within
// the given organization (and its tenant). Without an organization, only
// platform-scoped bindings apply.
func (b RoleBinding) AppliesTo(orgId, tenantId uuid.UUID) bool {
	if orgId == uuid.Nil {
		return b.OrgId == uuid.Nil && b.TenantId == uuid.Nil
	}

	return b.OrgId == orgId || (b.TenantId != uuid.Nil && b.TenantId == tenantId)
}

func ScopeRoleBindings(bindings []RoleBinding, orgId, tenantId uuid.UUID) []RoleBinding {
	scoped := make([]RoleBinding, 0, len(bindings))

	for _, binding := range bindings {
		if binding.AppliesTo(orgId, tenantId) {
			scoped = append(scoped, binding)
		}
	}

	return scoped
}

func (b RoleBinding) RoleName() Role {
	return b.Role
}

func NewNativeRoleBinding(role Role) RoleBinding {
	return RoleBinding{Role: role, Permissions: role.Permissions()}
}

func RoleNames(bindings []RoleBinding) []Role {
	roles := make([]Role, 0, len(bindings))

	for _, binding := range bindings {
		if role := binding.RoleName(); role != "" && !slices.Contains(roles, role) {
			roles = append(roles, role)
		}
	}

	return roles
}

func NativeRoleBindings(roles []Role) []RoleBinding {
	bindings := make([]RoleBinding, 0, len(roles))

	for _, role := range roles {
		bindings = append(bindings, NewNativeRoleBinding(role))
	}

	return bindings
}

// LegacyRoleValue returns the value stored in the singular role column for
// rollback compatibility. The legacy schema can represent only one native
// role, so the first native binding is used.
func LegacyRoleValue(bindings []RoleBinding) int {
	values := map[Role]int{
		SYSTEM:       0,
		VIEWER:       1,
		BUILDER:      2,
		PUBLISHER:    3,
		ADMIN:        4,
		API_CLIENT:   5,
		MARBLE_ADMIN: 6,
		ANALYST:      9,
	}

	for _, binding := range bindings {
		if value, ok := values[binding.Role]; ok {
			return value
		}
	}

	return 0
}

type Role string

// Do not remove or reorder entries here, even if a role if deleted, since the
// value is used for identity.
const (
	SYSTEM       Role = "SYSTEM"
	VIEWER       Role = "VIEWER"
	BUILDER      Role = "BUILDER"
	PUBLISHER    Role = "PUBLISHER"
	ADMIN        Role = "ADMIN"
	API_CLIENT   Role = "API_CLIENT"
	MARBLE_ADMIN Role = "MARBLE_ADMIN"
	ANALYST      Role = "ANALYST"
	TENANT_ADMIN Role = "TENANT_ADMIN"
)

func GetValidUserRoles() []Role {
	return []Role{
		VIEWER,
		BUILDER,
		PUBLISHER,
		ADMIN,
		MARBLE_ADMIN,
		ANALYST,
	}
}

func GetValidOrganizationGrantRoles() []Role {
	return []Role{
		VIEWER,
		BUILDER,
		PUBLISHER,
		ADMIN,
		ANALYST,
	}
}

func (r Role) String() string {
	return string(r)
}

// RoleFromString parses a native role name, returning an empty role for
// unknown or internal (SYSTEM) roles.
func RoleFromString(s string) Role {
	switch role := Role(s); role {
	case VIEWER, BUILDER, PUBLISHER, ADMIN, API_CLIENT, MARBLE_ADMIN, ANALYST, TENANT_ADMIN:
		return role
	}
	return ""
}

func (r Role) Permissions() []Permission {
	permissions := ROLES_PERMISSIONS[r]
	if permissions == nil {
		return []Permission{}
	}
	return permissions
}

func (r Role) HasPermission(permission Permission) bool {
	return slices.Contains(r.Permissions(), permission)
}
