package models

import (
	"slices"
)

type Role string

// Do not remove or reorder entries here, even if a role if deleted, since the
// value is used for identity.
const (
	NO_ROLE      Role = ""
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

// legacyRoleValues are the values roles had as integers, still stored in the
// role column of users and API keys.
var legacyRoleValues = map[Role]int{
	NO_ROLE:      0,
	VIEWER:       1,
	BUILDER:      2,
	PUBLISHER:    3,
	ADMIN:        4,
	API_CLIENT:   5,
	MARBLE_ADMIN: 6,
	ANALYST:      9,
	SYSTEM:       10,
	TENANT_ADMIN: 11,
}

// LegacyValue returns the integer value of the role, for the legacy role
// column of users and API keys.
func (r Role) LegacyValue() int {
	return legacyRoleValues[r]
}

// RoleFromLegacyValue returns the role stored as an integer in the legacy role
// column of users and API keys.
func RoleFromLegacyValue(value int) Role {
	for role, legacyValue := range legacyRoleValues {
		if legacyValue == value {
			return role
		}
	}

	return NO_ROLE
}

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
	return NO_ROLE
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
