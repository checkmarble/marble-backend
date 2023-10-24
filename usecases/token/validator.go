package token

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
)

type keyAndOrganizationGetter interface {
	GetApiKeyByKey(ctx context.Context, key string) (models.ApiKey, error)
	GetOrganizationByID(ctx context.Context, organizationID string) (models.Organization, error)
}

type marbleTokenValidator interface {
	ValidateMarbleToken(marbleToken string) (models.Credentials, error)
}

type Validator struct {
	getter    keyAndOrganizationGetter
	validator marbleTokenValidator
}

func (v *Validator) fromAPIKey(ctx context.Context, key string) (models.Credentials, error) {
	apiKey, err := v.getter.GetApiKeyByKey(ctx, key)
	if err != nil {
		return models.Credentials{}, fmt.Errorf("getter.GetApiKeyByKey error: %w", err)
	}

	organization, err := v.getter.GetOrganizationByID(ctx, apiKey.OrganizationId)
	if err != nil {
		return models.Credentials{}, fmt.Errorf("getter.GetOrganizationByID error: %w", err)
	}
	name := fmt.Sprintf("ApiKey Of %s", organization.Name)
	credentials := models.NewCredentialWithApiKey(apiKey.OrganizationId, apiKey.Role, name)
	return credentials, nil
}

func (v *Validator) Validate(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error) {
	if apiKey != "" {
		return v.fromAPIKey(ctx, apiKey)
	}
	return v.validator.ValidateMarbleToken(marbleToken)
}

func NewValidator(getter keyAndOrganizationGetter, validator marbleTokenValidator) *Validator {
	return &Validator{
		getter:    getter,
		validator: validator,
	}
}