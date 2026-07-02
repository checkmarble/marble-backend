package models

import (
	"slices"

	"github.com/google/uuid"
)

type RbacRole struct {
	Id          uuid.UUID
	OrgId       uuid.UUID
	Name        string
	Permissions []RbacPermission
}

type RbacPermission struct {
	Id        uuid.UUID
	OrgId     uuid.UUID
	Name      string
	Condition *string
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
