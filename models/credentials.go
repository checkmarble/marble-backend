package models

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
	OrganizationId string
	PartnerId      *string
	Role           Role
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
		PartnerId:      u.PartnerId,
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
		PartnerId:      k.PartnerId,
		Role:           k.Role,
	}
}
