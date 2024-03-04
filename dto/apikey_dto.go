package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type ApiKey struct {
	Id             string    `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	OrganizationId string    `json:"organization_id"`
	Description    string    `json:"description"`
	Role           string    `json:"role"`
}

func AdaptApiKeyDto(apiKey models.ApiKey) ApiKey {
	return ApiKey{
		Id:             apiKey.Id,
		CreatedAt:      apiKey.CreatedAt,
		OrganizationId: apiKey.OrganizationId,
		Description:    apiKey.Description,
		Role:           apiKey.Role.String(),
	}
}

type CreatedApiKey struct {
	ApiKey
	Key string `json:"key"`
}

func AdaptCreatedApiKeyDto(apiKey models.ApiKey) CreatedApiKey {
	return CreatedApiKey{
		ApiKey: AdaptApiKeyDto(apiKey),
		Key:    apiKey.Key,
	}
}

type CreateApiKeyBody struct {
	Description string `json:"description"`
	Role        string `json:"role"`
}
