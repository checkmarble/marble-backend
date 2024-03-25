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
	ActorIdentity  Identity `json:"actor_identity"`
	OrganizationId string   `json:"organization_id"`
	PartnerId      string   `json:"partner_id"`
	Permissions    []string `json:"permissions"`
	Role           string   `json:"role"`
}

func AdaptCredentialDto(creds models.Credentials) Credentials {
	permissions := pure_utils.Map(creds.Role.Permissions(),
		func(p models.Permission) string { return p.String() })

	return Credentials{
		ActorIdentity: Identity{
			UserId:     string(creds.ActorIdentity.UserId),
			Email:      creds.ActorIdentity.Email,
			FirstName:  creds.ActorIdentity.FirstName,
			LastName:   creds.ActorIdentity.LastName,
			ApiKeyName: creds.ActorIdentity.ApiKeyName,
		},
		OrganizationId: creds.OrganizationId,
		PartnerId:      creds.PartnerId,
		Permissions:    permissions,
		Role:           creds.Role.String(),
	}
}

func AdaptCredential(dto Credentials) models.Credentials {
	return models.Credentials{
		ActorIdentity: models.Identity{
			UserId:     models.UserId(dto.ActorIdentity.UserId),
			Email:      dto.ActorIdentity.Email,
			FirstName:  dto.ActorIdentity.FirstName,
			LastName:   dto.ActorIdentity.LastName,
			ApiKeyName: dto.ActorIdentity.ApiKeyName,
		},
		OrganizationId: dto.OrganizationId,
		PartnerId:      dto.PartnerId,
		Role:           models.RoleFromString(dto.Role),
	}
}
