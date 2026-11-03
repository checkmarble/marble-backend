package token

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type keyAndOrganizationGetter interface {
	GetUserByID(ctx context.Context, userID string) (models.User, error)
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
		return models.Credentials{}, errors.Errorf("getter.GetApiKeyByHash error: %w", err)
	}

	organization, err := v.getter.GetOrganizationByID(ctx, apiKey.OrganizationId)
	if err != nil {
		return models.Credentials{}, errors.Errorf("getter.GetOrganizationByID error: %w", err)
	}

	apiKey.DisplayString = fmt.Sprintf("Api key %s*** of %s", apiKey.Prefix, organization.Name)
	credentials := apiKey.IntoCredentials()
	grants, err := v.getter.ActiveGrantsForPrincipal(ctx, "api_key", apiKey.Id)
	if err != nil {
		return models.Credentials{}, errors.Errorf("ActiveGrantsForPrincipal error: %w", err)
	}
	credentials.Roles = []models.Role{}
	for _, grant := range grants {
		if (grant.OrganizationId == apiKey.OrganizationId || grant.TenantId == organization.TenantId) &&
			!slices.Contains(credentials.Roles, grant.Role) {
			credentials.Roles = append(credentials.Roles, grant.Role)
		}
	}
	slices.Sort(credentials.Roles)
	if len(credentials.Roles) == 0 {
		return models.Credentials{}, errors.Errorf("%w: API key has no active grant", models.UnAuthorizedError)
	}
	credentials.TenantId = organization.TenantId
	return credentials, nil
}

func (v *Validator) ValidateTokenOrKey(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error) {
	if apiKey != "" {
		return v.fromAPIKey(ctx, apiKey)
	}

	credentials, err := v.validator.ValidateMarbleToken(marbleToken)
	if err != nil {
		return models.Credentials{}, err
	}
	if credentials.OrganizationId != uuid.Nil {
		organization, err := v.getter.GetOrganizationByID(ctx, credentials.OrganizationId)
		if err != nil {
			return models.Credentials{}, errors.Wrap(err, "getter.GetOrganizationByID error")
		}
		if credentials.AuthTenantId != uuid.Nil && organization.TenantId != credentials.AuthTenantId {
			return models.Credentials{}, errors.Errorf("%w: organization is outside the authentication tenant", models.UnAuthorizedError)
		}
		credentials.TenantId = organization.TenantId
	}
	if credentials.AuthTenantId != uuid.Nil || credentials.HasRole(models.TENANT_ADMIN) {
		return v.revalidateUserGrants(ctx, credentials)
	}
	return credentials, nil
}

func (v *Validator) revalidateUserGrants(ctx context.Context, credentials models.Credentials) (models.Credentials, error) {
	if credentials.ActorIdentity.UserId == "" {
		return credentials, nil
	}
	user, err := v.getter.GetUserByID(ctx, string(credentials.ActorIdentity.UserId))
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return models.Credentials{}, errors.Wrap(models.UnAuthorizedError, "identity no longer active")
		}
		return models.Credentials{}, errors.Wrap(err, "loading identity")
	}
	if credentials.OrganizationId == uuid.Nil {
		credentials.TenantId = user.TenantId
	}
	if (credentials.AuthTenantId != uuid.Nil && user.TenantId != credentials.AuthTenantId) ||
		(credentials.HasRole(models.TENANT_ADMIN) && (credentials.TenantId == uuid.Nil || user.TenantId != credentials.TenantId)) {
		return models.Credentials{}, models.UnAuthorizedError
	}
	grants, err := v.getter.ActiveGrantsForPrincipal(ctx, "user", string(credentials.ActorIdentity.UserId))
	if err != nil {
		return models.Credentials{}, err
	}
	if len(grants) == 0 {
		return models.Credentials{}, models.UnAuthorizedError
	}
	roles := []models.Role{}
	organizationAccess := credentials.OrganizationId == uuid.Nil
	for _, grant := range grants {
		applicable := false
		if credentials.OrganizationId != uuid.Nil {
			if grant.OrganizationId == credentials.OrganizationId {
				organizationAccess = true
				applicable = true
			}
			applicable = applicable || (grant.TenantId == credentials.TenantId && grant.TenantId != uuid.Nil && grant.Role == models.TENANT_ADMIN)
		} else {
			applicable = (grant.TenantId == uuid.Nil && grant.OrganizationId == uuid.Nil && credentials.AuthTenantId == uuid.Nil) ||
				(grant.TenantId != uuid.Nil && grant.TenantId == credentials.TenantId && grant.Role == models.TENANT_ADMIN)
		}
		// Revocation takes effect immediately; newly granted privileges require a new token.
		if applicable && slices.Contains(credentials.Roles, grant.Role) && !slices.Contains(roles, grant.Role) {
			roles = append(roles, grant.Role)
		}
	}
	if !organizationAccess || (credentials.OrganizationId != uuid.Nil && len(roles) == 0) || (credentials.HasRole(models.TENANT_ADMIN) && !slices.Contains(roles, models.TENANT_ADMIN)) || (credentials.HasRole(models.MARBLE_ADMIN) && !slices.Contains(roles, models.MARBLE_ADMIN)) {
		return models.Credentials{}, models.UnAuthorizedError
	}
	slices.Sort(roles)
	credentials.Roles = roles
	return credentials, nil
}

func NewValidator(getter keyAndOrganizationGetter, validator marbleTokenValidator) *Validator {
	return &Validator{
		getter:    getter,
		validator: validator,
	}
}
