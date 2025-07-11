package models

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
	OrganizationId string
	PartnerId      *string
	Role           Role
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
		PartnerId:      user.PartnerId,
		Role:           user.Role,
	}
}

func NewCredentialWithApiKey(key ApiKey, apiKeyName string) Credentials {
	return Credentials{
		ActorIdentity: Identity{
			ApiKeyId:   key.Id,
			ApiKeyName: apiKeyName,
		},
		OrganizationId: key.OrganizationId,
		PartnerId:      key.PartnerId,
		Role:           key.Role,
	}
}
