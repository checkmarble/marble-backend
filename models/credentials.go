package models

import (
	"fmt"
)

type Identity struct {
	UserId     UserId
	Email      string
	FirstName  string
	LastName   string
	ApiKeyName string
}

type Credentials struct {
	OrganizationId string
	Role           Role
	ActorIdentity  Identity // email or api key, for audit log
}

func (c Credentials) ActorIdentityDescription() string {
	return fmt.Sprintf("%s%s (%s)", c.ActorIdentity.Email, c.ActorIdentity.ApiKeyName, c.Role.String())
}

func NewCredentialWithUser(user User) Credentials {
	return Credentials{
		OrganizationId: user.OrganizationId,
		Role:           user.Role,
		ActorIdentity: Identity{
			UserId:    user.UserId,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		},
	}
}

func NewCredentialWithApiKey(organizationId string, role Role, apiKeyName string) Credentials {
	return Credentials{
		OrganizationId: organizationId,
		Role:           role,
		ActorIdentity: Identity{
			ApiKeyName: apiKeyName,
		},
	}
}
