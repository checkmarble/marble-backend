package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type Identity struct {
	UserId    string `json:"user_id,omitempty"`
	Email     string `json:"email,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`

	ApiKeyId   string `json:"api_key_id,omitempty"`
	ApiKeyName string `json:"api_key_name,omitempty"`
}

type Credentials struct {
	ActorIdentity  Identity            `json:"actor_identity"`
	OrganizationId uuid.UUID           `json:"organization_id"`
	Permissions    []models.Permission `json:"permissions"`
}

func AdaptCredentialDto(creds models.Credentials) (Credentials, error) {
	return Credentials{
		ActorIdentity: Identity{
			UserId:     string(creds.ActorIdentity.UserId),
			Email:      creds.ActorIdentity.Email,
			FirstName:  creds.ActorIdentity.FirstName,
			LastName:   creds.ActorIdentity.LastName,
			ApiKeyId:   creds.ActorIdentity.ApiKeyId,
			ApiKeyName: creds.ActorIdentity.ApiKeyName,
		},
		OrganizationId: creds.OrganizationId,
		Permissions:    creds.Permissions,
	}, nil
}

func AdaptCredential(dto Credentials) models.Credentials {
	return models.Credentials{
		ActorIdentity: models.Identity{
			UserId:     models.UserId(dto.ActorIdentity.UserId),
			Email:      dto.ActorIdentity.Email,
			FirstName:  dto.ActorIdentity.FirstName,
			LastName:   dto.ActorIdentity.LastName,
			ApiKeyId:   dto.ActorIdentity.ApiKeyId,
			ApiKeyName: dto.ActorIdentity.ApiKeyName,
		},
		OrganizationId: dto.OrganizationId,
		Permissions:    dto.Permissions,
	}
}
