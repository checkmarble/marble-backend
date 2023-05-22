package models

type Role int

const (
	NO_ROLE Role = iota
	READER
	BUILDER
	PUBLISHER
	ADMIN
	API_KEY
	MARBLE_ADMIN
)

func (r Role) String() string {
	return [...]string{"NO_ROLE", "READER", "BUILDER", "PUBLISHER", "ADMIN", "API_KEY", "MARBLE_ADMIN"}[r]
}

func RoleFromString(s string) Role {
	switch s {
	case "READER":
		return READER
	case "BUILDER":
		return BUILDER
	case "PUBLISHER":
		return PUBLISHER
	case "ADMIN":
		return ADMIN
	case "API_KEY":
		return API_KEY
	case "MARBLE_ADMIN":
		return MARBLE_ADMIN
	}
	return NO_ROLE
}
