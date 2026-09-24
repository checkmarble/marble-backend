package models

import (
	"slices"
	"time"

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
	RoleBindings   []RoleBinding
	Permissions    []Permission
}

func (c Credentials) HasRole(roles ...Role) bool {
	now := time.Now()

	for _, binding := range c.RoleBindings {
		if binding.IsActive(now) && slices.Contains(roles, binding.RoleName()) {
			return true
		}
	}

	return false
}

func (c Credentials) HasPermission(perm Permission) bool {
	if len(c.RoleBindings) > 0 {
		now := time.Now()

		for _, binding := range c.RoleBindings {
			if binding.IsActive(now) && slices.Contains(binding.Permissions, perm) {
				return true
			}
		}

		return false
	}

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
		RoleBindings:   u.RoleBindings,
	}
}

func (k ApiKey) IntoCredentials() Credentials {
	return Credentials{
		ActorIdentity: Identity{
			ApiKeyId:   k.Id,
			ApiKeyName: k.DisplayString,
		},
		OrganizationId: k.OrganizationId,
		RoleBindings:   k.RoleBindings,
	}
}
