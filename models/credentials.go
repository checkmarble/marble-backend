package models

type Identity struct {
	UserId     UserId
	Email      string
	ApiKeyName string
}

type Credentials struct {
	OrganizationId string
	Role           Role
	ActorIdentity  Identity // email or api key, for audit log
}

func NewCredentialWithUser(organizationId string, role Role, userId UserId, userEmail string) Credentials {
	return Credentials{
		OrganizationId: organizationId,
		Role:           role,
		ActorIdentity: Identity{
			UserId: userId,
			Email:  userEmail,
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
