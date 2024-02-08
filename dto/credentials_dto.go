package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type Identity struct {
	UserId     string `json:"user_id,omitempty"`
	Email      string `json:"email,omitempty"`
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	ApiKeyName string `json:"api_key_name,omitempty"`
}

type Credentials struct {
	OrganizationId string   `json:"organization_id"`
	Role           string   `json:"role"`
	ActorIdentity  Identity `json:"actor_identity"`
	Permissions    []string `json:"permissions"`
}

func AdaptCredentialDto(creds models.Credentials) Credentials {
	permissions := pure_utils.Map(creds.Role.Permissions(), func(p models.Permission) string { return p.String() })

	return Credentials{
		OrganizationId: creds.OrganizationId,
		Role:           creds.Role.String(),
		ActorIdentity: Identity{
			UserId:     string(creds.ActorIdentity.UserId),
			Email:      creds.ActorIdentity.Email,
			FirstName:  creds.ActorIdentity.FirstName,
			LastName:   creds.ActorIdentity.LastName,
			ApiKeyName: creds.ActorIdentity.ApiKeyName,
		},
		Permissions: permissions,
	}
}

func AdaptCredential(dto Credentials) models.Credentials {
	return models.Credentials{
		OrganizationId: dto.OrganizationId,
		Role:           models.RoleFromString(dto.Role),
		ActorIdentity: models.Identity{
			UserId:     models.UserId(dto.ActorIdentity.UserId),
			Email:      dto.ActorIdentity.Email,
			FirstName:  dto.ActorIdentity.FirstName,
			LastName:   dto.ActorIdentity.LastName,
			ApiKeyName: dto.ActorIdentity.ApiKeyName,
		},
	}
}
