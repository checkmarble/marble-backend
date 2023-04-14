package api

import "github.com/golang-jwt/jwt/v5"

type Credentials struct {
	RefreshToken string `json:"refresh_token"`
}

type Role int

const (
	READER Role = iota
	BUILDER
	PUBLISHER
	ADMIN
)

func (r Role) String() string {
	return [...]string{"READER", "BUILDER", "PUBLISHER", "ADMIN"}[r]
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
	}
	return READER
}

type TokenType string

const (
	ApiToken      TokenType = "API"
	UserToken     TokenType = "USER"
	InternalToken TokenType = "INTERNAL"
)

// We add jwt.RegisteredClaims as an embedded type, to provide fields like expiry time
type Claims struct {
	OrganizationId string `json:"organization_id"`
	Type           string `json:"type"`
	Role           string `json:"role"`
	jwt.RegisteredClaims
}
