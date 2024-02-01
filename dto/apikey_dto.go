package dto

import "github.com/checkmarble/marble-backend/models"

type ApiKey struct {
	Id             string `json:"id"`
	OrganizationId string `json:"organization_id"`
	Key            string `json:"key"`
	Description    string `json:"description"`
	Role           string `json:"role"`
}

func AdaptApiKeyDto(apiKey models.ApiKey) ApiKey {
	return ApiKey{
		Id:             apiKey.Id,
		OrganizationId: apiKey.OrganizationId,
		Key:            apiKey.Key,
		Description:    apiKey.Description,
		Role:           apiKey.Role.String(),
	}
}
