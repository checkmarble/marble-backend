package token

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type keyAndOrganizationGetter interface {
	GetApiKeyByHash(ctx context.Context, hash []byte) (models.ApiKey, error)
	GetOrganizationByID(ctx context.Context, organizationID uuid.UUID) (models.Organization, error)
	ActiveGrantsForPrincipal(ctx context.Context, principalType, principalID string) ([]models.Grant, error)
}

type marbleTokenValidator interface {
	ValidateMarbleToken(marbleToken string) (models.Credentials, error)
}

type Validator struct {
	getter    keyAndOrganizationGetter
	validator marbleTokenValidator
}

func (v *Validator) fromAPIKey(ctx context.Context, key string) (models.Credentials, error) {
	hash := sha256.Sum256([]byte(key))
	apiKey, err := v.getter.GetApiKeyByHash(ctx, hash[:])
	if err != nil {
		return models.Credentials{}, fmt.Errorf("getter.GetApiKeyByHash error: %w", err)
	}

	organization, err := v.getter.GetOrganizationByID(ctx, apiKey.OrganizationId)
	if err != nil {
		return models.Credentials{}, fmt.Errorf("getter.GetOrganizationByID error: %w", err)
	}

	apiKey.DisplayString = fmt.Sprintf("Api key %s*** of %s", apiKey.Prefix, organization.Name)
	credentials := apiKey.IntoCredentials()
	credentials.TenantId = organization.TenantId
	grants, err := v.getter.ActiveGrantsForPrincipal(ctx, "api_key", apiKey.Id)
	if err != nil {
		return models.Credentials{}, fmt.Errorf("ActiveGrantsForPrincipal error: %w", err)
	}
	// Roles are the union of the legacy role (api_keys.role) and the active
	// grants, until the authority is switched to grants-only with the backfill.
	credentials.Roles = []models.Role{apiKey.Role}
	for _, grant := range grants {
		if (grant.OrganizationId == apiKey.OrganizationId || grant.TenantId == organization.TenantId) &&
			!slices.Contains(credentials.Roles, grant.Role) {
			credentials.Roles = append(credentials.Roles, grant.Role)
		}
	}
	credentials.Role = apiKey.Role

	return credentials, nil
}

func (v *Validator) ValidateTokenOrKey(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error) {
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
