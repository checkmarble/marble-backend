package models

import (
	"slices"

	"github.com/google/uuid"
)

type IntoCredentials interface {
	IntoCredentials() Credentials
}

type Identity struct {
	UserId     UserId
	Email      string
	FirstName  string
	LastName   string
	ApiKeyId   string
	ApiKeyName string
}

type Credentials struct {
	ActorIdentity  Identity // email or api key, for audit log
	OrganizationId uuid.UUID
	Roles          []Role
	Permissions    []Permission
}

func (c Credentials) HasRole(roles ...Role) bool {
	for _, role := range roles {
		if slices.Contains(c.Roles, role) {
			return true
		}
	}
	return false
}

func (c Credentials) HasPermission(perm Permission) bool {
	return slices.Contains(c.Permissions, perm)
}

func (u User) IntoCredentials() Credentials {
	return Credentials{
		ActorIdentity: Identity{
			UserId:    u.UserId,
			Email:     u.Email,
			FirstName: u.FirstName,
			LastName:  u.LastName,
		},
		OrganizationId: u.OrganizationId,
		Roles:          u.Roles,
	}
}

func (k ApiKey) IntoCredentials() Credentials {
	return Credentials{
		ActorIdentity: Identity{
			ApiKeyId:   k.Id,
			ApiKeyName: k.DisplayString,
		},
		OrganizationId: k.OrganizationId,
		Roles:          k.Roles,
	}
}
