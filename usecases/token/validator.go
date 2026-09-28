package token

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type keyAndOrganizationGetter interface {
	GetApiKeyByHash(ctx context.Context, hash []byte) (models.ApiKey, error)
	GetOrganizationByID(ctx context.Context, organizationID uuid.UUID) (models.Organization, error)
	ActiveGrantsForPrincipal(ctx context.Context, principalType, principalID string) ([]models.RoleBinding, error)
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

	return v.withRoleBindings(ctx, credentials, organization.TenantId)
}

func (v *Validator) fromMarbleToken(ctx context.Context, marbleToken string) (models.Credentials, error) {
	credentials, err := v.validator.ValidateMarbleToken(marbleToken)
	if err != nil {
		return models.Credentials{}, err
	}

	// Tokens do not carry the tenant, which is needed to apply tenant-scoped
	// grants.
	tenantId := uuid.Nil
	if credentials.OrganizationId != uuid.Nil {
		organization, err := v.getter.GetOrganizationByID(ctx, credentials.OrganizationId)
		if err != nil {
			return models.Credentials{}, fmt.Errorf("getter.GetOrganizationByID error: %w", err)
		}
		tenantId = organization.TenantId
	}

	return v.withRoleBindings(ctx, credentials, tenantId)
}

// withRoleBindings resolves the principal's role bindings from its active
// grants, so that grant changes apply immediately, regardless of the roles
// that were current when a token was issued.
func (v *Validator) withRoleBindings(ctx context.Context, credentials models.Credentials, tenantId uuid.UUID) (models.Credentials, error) {
	var principalType, principalId string

	switch {
	case credentials.ActorIdentity.UserId != "":
		principalType, principalId = "user", string(credentials.ActorIdentity.UserId)
	case credentials.ActorIdentity.ApiKeyId != "":
		principalType, principalId = "api_key", credentials.ActorIdentity.ApiKeyId
	default:
		return models.Credentials{}, fmt.Errorf("%w: credentials do not identify a principal", models.UnAuthorizedError)
	}

	grants, err := v.getter.ActiveGrantsForPrincipal(ctx, principalType, principalId)
	if err != nil {
		return models.Credentials{}, fmt.Errorf("ActiveGrantsForPrincipal error: %w", err)
	}

	if len(grants) == 0 {
		return models.Credentials{}, fmt.Errorf("%w: principal has no active grant", models.UnAuthorizedError)
	}

	credentials.RoleBindings = models.ScopeRoleBindings(grants, credentials.OrganizationId, tenantId)

	// API keys always belong to an organization, and are useless without a
	// grant applying to it.
	if principalType == "api_key" && len(credentials.RoleBindings) == 0 {
		return models.Credentials{}, fmt.Errorf("%w: API key has no applicable grant", models.UnAuthorizedError)
	}

	credentials.Permissions = models.RoleBindingsPermissions(credentials.RoleBindings)

	return credentials, nil
}

func (v *Validator) ValidateTokenOrKey(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error) {
	if apiKey != "" {
		return v.fromAPIKey(ctx, apiKey)
	}
	return v.fromMarbleToken(ctx, marbleToken)
}

func NewValidator(getter keyAndOrganizationGetter, validator marbleTokenValidator) *Validator {
	return &Validator{
		getter:    getter,
		validator: validator,
	}
}
