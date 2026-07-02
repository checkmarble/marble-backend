package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type ApiKey struct {
	Id             string        `json:"id"`
	CreatedAt      time.Time     `json:"created_at"`
	Description    string        `json:"description"`
	OrganizationId uuid.UUID     `json:"organization_id"`
	Prefix         string        `json:"prefix"`
	Roles          []models.Role `json:"roles"`
}

func AdaptApiKeyDto(apiKey models.ApiKey) ApiKey {
	return ApiKey{
		Id:             apiKey.Id,
		CreatedAt:      apiKey.CreatedAt,
		Description:    apiKey.Description,
		OrganizationId: apiKey.OrganizationId,
		Prefix:         apiKey.Prefix,
		Roles:          apiKey.Roles,
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
	Description string `json:"description"`
	Role        string `json:"role"`
}
