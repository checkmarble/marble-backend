package dto

import (
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type Identity struct {
	UserId     string `json:"user_id,omitempty"`
	Email      string `json:"email,omitempty"`
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	ApiKeyName string `json:"api_key_name,omitempty"`
}

type Credentials struct {
	ActorIdentity  Identity  `json:"actor_identity"`
	OrganizationId uuid.UUID `json:"organization_id"`
	TenantId       uuid.UUID `json:"tenant_id"`
	Permissions    []string  `json:"permissions"`
	Role           string    `json:"role,omitempty"`
	Roles          []string  `json:"roles,omitempty"`
}

func AdaptCredentialDto(creds models.Credentials) (Credentials, error) {
	permissions, err := pure_utils.MapErr(
		permissionsForCredentials(creds),
		func(p models.Permission) (string, error) { return p.String() },
	)
	if err != nil {
		return Credentials{}, err
	}

	return Credentials{
		ActorIdentity: Identity{
			UserId:     string(creds.ActorIdentity.UserId),
			Email:      creds.ActorIdentity.Email,
			FirstName:  creds.ActorIdentity.FirstName,
			LastName:   creds.ActorIdentity.LastName,
			ApiKeyName: creds.ActorIdentity.ApiKeyName,
		},
		OrganizationId: creds.OrganizationId,
		TenantId:       creds.TenantId,
		Permissions:    permissions,
		Role:           creds.Role.String(),
		Roles:          pure_utils.Map(creds.Roles, func(role models.Role) string { return role.String() }),
	}, nil
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
		TenantId:       dto.TenantId,
		Role:           models.RoleFromString(dto.Role),
		Roles:          pure_utils.Map(dto.Roles, models.RoleFromString),
	}
}

func permissionsForCredentials(creds models.Credentials) []models.Permission {
	permissions := []models.Permission{}
	for _, role := range creds.Roles {
		for _, permission := range role.Permissions() {
			if !slices.Contains(permissions, permission) {
				permissions = append(permissions, permission)
			}
		}
	}
	if len(creds.Roles) == 0 {
		return creds.Role.Permissions()
	}
	return permissions
}
