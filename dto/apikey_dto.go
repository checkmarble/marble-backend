package dto

import "github.com/checkmarble/marble-backend/models"

type ApiKey struct {
	Id             string `json:"id"`
	OrganizationId string `json:"organization_id"`
	Description    string `json:"description"`
	Role           string `json:"role"`
}

func AdaptApiKeyDto(apiKey models.ApiKey) ApiKey {
	return ApiKey{
		Id:             apiKey.Id,
		OrganizationId: apiKey.OrganizationId,
		Description:    apiKey.Description,
		Role:           apiKey.Role.String(),
	}
}

type CreatedApiKey struct {
	ApiKey
	Key string `json:"key"`
}

func AdaptCreatedApiKeyDto(apiKey models.CreatedApiKey) CreatedApiKey {
	return CreatedApiKey{
		ApiKey: AdaptApiKeyDto(apiKey.ApiKey),
		Key:    apiKey.Value,
	}
}

type CreateApiKeyBody struct {
	Description string `json:"description"`
	Role        string `json:"role"`
}
