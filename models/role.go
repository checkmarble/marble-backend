package models

import (
	"slices"
)

type Role int

// Do not remove or reorder entries here, even if a role if deleted, since the
// value is used for identity.
const (
	NO_ROLE Role = iota
	VIEWER
	BUILDER
	PUBLISHER
	ADMIN
	API_CLIENT
	MARBLE_ADMIN
	TRANSFER_CHECK_API_CLIENT
	TRANSFER_CHECK_USER
	ANALYST
)

func GetValidUserRoles() []Role {
	return []Role{
		VIEWER,
		BUILDER,
		PUBLISHER,
		ADMIN,
		MARBLE_ADMIN,
		TRANSFER_CHECK_USER,
		ANALYST,
	}
}

func GetValidTransfercheckUserRoles() []Role {
	return []Role{
		TRANSFER_CHECK_USER,
	}
}

func (r Role) String() string {
	switch r {
	case NO_ROLE:
		return "NO_ROLE"
	case VIEWER:
		return "VIEWER"
	case ANALYST:
		return "ANALYST"
	case BUILDER:
		return "BUILDER"
	case PUBLISHER:
		return "PUBLISHER"
	case ADMIN:
		return "ADMIN"
	case API_CLIENT:
		return "API_CLIENT"
	case MARBLE_ADMIN:
		return "MARBLE_ADMIN"
	case TRANSFER_CHECK_API_CLIENT:
		return "TRANSFER_CHECK_API_CLIENT"
	case TRANSFER_CHECK_USER:
		return "TRANSFER_CHECK_USER"
	default:
		return "UNKNOWN_ROLE"
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

func RoleFromString(s string) Role {
	switch s {
	case "VIEWER":
		return VIEWER
	case "ANALYST":
		return ANALYST
	case "BUILDER":
		return BUILDER
	case "PUBLISHER":
		return PUBLISHER
	case "ADMIN":
		return ADMIN
	case "API_CLIENT":
		return API_CLIENT
	case "MARBLE_ADMIN":
		return MARBLE_ADMIN
	case "TRANSFER_CHECK_API_CLIENT":
		return TRANSFER_CHECK_API_CLIENT
	case "TRANSFER_CHECK_USER":
		return TRANSFER_CHECK_USER
	}
	return NO_ROLE
}
