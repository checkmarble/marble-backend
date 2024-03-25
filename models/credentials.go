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
	ActorIdentity  Identity // email or api key, for audit log
	OrganizationId string
	PartnerId      string
	Role           Role
}

func (c Credentials) ActorIdentityDescription() string {
	return fmt.Sprintf("%s%s (%s)", c.ActorIdentity.Email, c.ActorIdentity.ApiKeyName, c.Role.String())
}

func NewCredentialWithUser(user User) Credentials {
	return Credentials{
		ActorIdentity: Identity{
			UserId:    user.UserId,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		},
		OrganizationId: user.OrganizationId,
		Role:           user.Role,
	}
}

func NewCredentialWithApiKey(organizationId string, partnerId string, role Role, apiKeyName string) Credentials {
	return Credentials{
		ActorIdentity: Identity{
			ApiKeyName: apiKeyName,
		},
		OrganizationId: organizationId,
		PartnerId:      partnerId,
		Role:           role,
	}
}
