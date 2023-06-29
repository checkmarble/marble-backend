package dto

import "marble/marble-backend/models"

type Identity struct {
	UserId     string `json:"user_id,omitempty"`
	Email      string `json:"email,omitempty"`
	ApiKeyName string `json:"api_key_name,omitempty"`
}

type Credentials struct {
	OrganizationId string   `json:"organization_id"`
	Role           string   `json:"role"`
	ActorIdentity  Identity `json:"actor_identity"`
}

func AdaptCredentialDto(creds models.Credentials) Credentials {
	return Credentials{
		OrganizationId: creds.OrganizationId,
		Role:           creds.Role.String(),
		ActorIdentity: Identity{
			UserId:     string(creds.ActorIdentity.UserId),
			Email:      creds.ActorIdentity.Email,
			ApiKeyName: creds.ActorIdentity.ApiKeyName,
		},
	}
}

func AdaptCredential(dto Credentials) models.Credentials {
	return models.Credentials{
		OrganizationId: dto.OrganizationId,
		Role:           models.RoleFromString(dto.Role),
		ActorIdentity: models.Identity{
			UserId:     models.UserId(dto.ActorIdentity.UserId),
			Email:      dto.ActorIdentity.Email,
			ApiKeyName: dto.ActorIdentity.ApiKeyName,
		},
	}
}
