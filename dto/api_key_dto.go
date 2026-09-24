package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type ApiKey struct {
	Id             string        `json:"id"`
	CreatedAt      time.Time     `json:"created_at"`
	Description    string        `json:"description"`
	OrganizationId uuid.UUID     `json:"organization_id"`
	Prefix         string        `json:"prefix"`
	RoleBindings   []RoleBinding `json:"roles"`
}

func AdaptApiKeyDto(apiKey models.ApiKey) ApiKey {
	return ApiKey{
		Id:             apiKey.Id,
		CreatedAt:      apiKey.CreatedAt,
		Description:    apiKey.Description,
		OrganizationId: apiKey.OrganizationId,
		Prefix:         apiKey.Prefix,
		RoleBindings:   pure_utils.Map(apiKey.RoleBindings, AdaptRoleBinding),
	}
}

type CreatedApiKey struct {
	ApiKey
	Key string `json:"key"`
}

func AdaptCreatedApiKeyDto(apiKey models.CreatedApiKey) CreatedApiKey {
	return CreatedApiKey{
		ApiKey: AdaptApiKeyDto(apiKey.ApiKey),
		Key:    apiKey.Key,
	}
}

type CreateApiKeyBody struct {
	Description  string        `json:"description"`
	RoleBindings []RoleBinding `json:"roles,omitempty"`
}
