package dto

import "github.com/checkmarble/marble-backend/models"

type ApiKey struct {
	ApiKeyId       string `json:"api_key_id"`
	OrganizationId string `json:"organization_id"`
	Key            string `json:"key"`
	Role           string `json:"role"`
}

func AdaptApiKeyDto(user models.ApiKey) ApiKey {
	return ApiKey{
		ApiKeyId:       string(user.ApiKeyId),
		OrganizationId: user.OrganizationId,
		Key:            user.Key,
		Role:           user.Role.String(),
	}
}
