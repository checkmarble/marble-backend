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
	RoleBindings   []RoleBinding
	// Permissions lists the permissions granted by the role bindings, for
	// information only (it is returned to the frontend). Authorization always
	// relies on RoleBindings.
	Permissions []Permission
}

func (c Credentials) HasRole(roles ...Role) bool {
	for _, binding := range c.RoleBindings {
		if slices.Contains(roles, binding.RoleName()) {
			return true
		}
	}

	return false
}

func (c Credentials) HasPermission(perm Permission) bool {
	for _, binding := range c.RoleBindings {
		if slices.Contains(binding.Permissions, perm) {
			return true
		}
	}

	return false
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

// RoleBindingsPermissions returns the deduplicated permissions granted by the
// bindings.
func RoleBindingsPermissions(bindings []RoleBinding) []Permission {
	permissions := make([]Permission, 0)

	for _, binding := range bindings {
		for _, permission := range binding.Permissions {
			if !slices.Contains(permissions, permission) {
				permissions = append(permissions, permission)
			}
		}
	}

	return permissions
}
