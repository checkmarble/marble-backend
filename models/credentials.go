package models

type Identity struct {
	UserId     string
	ApiKeyName string
}

type Credentials struct {
	OrganizationId string
	Role           Role
	ActorIdentity  Identity // email or api key, for audit log
}

func NewCredentialWithUser(organizationId string, role Role, userId string) Credentials {
	return Credentials{
		OrganizationId: organizationId,
		Role:           role,
		ActorIdentity: Identity{
			UserId: userId,
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
