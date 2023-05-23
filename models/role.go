package models

type Role int

const (
	NO_ROLE Role = iota
	VIEWER
	BUILDER
	PUBLISHER
	ADMIN
	API_CLIENT
	MARBLE_ADMIN
)

func (r Role) String() string {
	return [...]string{"NO_ROLE", "VIEWER", "BUILDER", "PUBLISHER", "ADMIN", "API_CLIENT", "MARBLE_ADMIN"}[r]
}

func (r Role) Permissions() []Permission {
	permissions := ROLES_PERMISSIOMS[r]
	if permissions == nil {
		return []Permission{}
	}
	return permissions
}

func RoleFromString(s string) Role {
	switch s {
	case "VIEWER":
		return VIEWER
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
	}
	return NO_ROLE
}
