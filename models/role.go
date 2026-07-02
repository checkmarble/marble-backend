package models

type Role string

// Do not remove or reorder entries here, even if a role if deleted, since the
// value is used for identity.
const (
	VIEWER       = "VIEWER"
	BUILDER      = "BUILDER"
	PUBLISHER    = "PUBLISHER"
	ADMIN        = "ADMIN"
	API_CLIENT   = "API_CLIENT"
	MARBLE_ADMIN = "MARBLE_ADMIN"
	ANALYST      = "ANALYST"
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
