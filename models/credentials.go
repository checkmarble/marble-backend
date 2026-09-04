package models

import "github.com/google/uuid"

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
	TenantId       uuid.UUID
	Role           Role // deprecated: kept while callers migrate to Roles
	Roles          []Role
}

func (c Credentials) HasRole(role Role) bool {
	if len(c.Roles) == 0 {
		return c.Role == role
	}
	for _, candidate := range c.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func (c Credentials) HasPermission(permission Permission) bool {
	if len(c.Roles) == 0 {
		return c.Role.HasPermission(permission)
	}
	for _, role := range c.Roles {
		if role.HasPermission(permission) {
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
		TenantId:       u.TenantId,
		Role:           u.Role,
	}
}

func (k ApiKey) IntoCredentials() Credentials {
	return Credentials{
		ActorIdentity: Identity{
			ApiKeyId:   k.Id,
			ApiKeyName: k.DisplayString,
		},
		OrganizationId: k.OrganizationId,
		Role:           k.Role,
	}
}
