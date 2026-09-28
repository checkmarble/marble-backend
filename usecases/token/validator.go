package token

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
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

func (v *Validator) fromAPIKey(ctx context.Context, key string, clientIp net.IP) (models.Credentials, error) {
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
	credentials.RoleBindingBundle.ClientIp = clientIp
	credentials.RoleBindingBundle.Location = organization.Timezone()

	return v.withRoleBindings(ctx, credentials, organization.TenantId)
}

func (v *Validator) fromMarbleToken(ctx context.Context, marbleToken string, clientIp net.IP) (models.Credentials, error) {
	credentials, err := v.validator.ValidateMarbleToken(marbleToken)
	if err != nil {
		return models.Credentials{}, err
	}
	credentials.RoleBindingBundle.ClientIp = clientIp

	// Tokens do not carry the tenant, which is needed to apply tenant-scoped
	// grants, nor the organization's time zone, used to evaluate caveats.
	tenantId := uuid.Nil
	credentials.RoleBindingBundle.Location = time.UTC
	if credentials.OrganizationId != uuid.Nil {
		organization, err := v.getter.GetOrganizationByID(ctx, credentials.OrganizationId)
		if err != nil {
			return models.Credentials{}, errors.Errorf("getter.GetOrganizationByID error: %w", err)
		}
		tenantId = organization.TenantId
		credentials.RoleBindingBundle.Location = organization.Timezone()
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
		return models.Credentials{}, errors.Errorf("%w: credentials do not identify a principal", models.UnAuthorizedError)
	}

	grants, err := v.getter.ActiveGrantsForPrincipal(ctx, principalType, principalId)
	if err != nil {
		return models.Credentials{}, errors.Errorf("ActiveGrantsForPrincipal error: %w", err)
	}

	if len(grants) == 0 {
		return models.Credentials{}, errors.Errorf("%w: principal has no active grant", models.UnAuthorizedError)
	}

	credentials.RoleBindings = models.ScopeRoleBindings(grants, credentials.OrganizationId, tenantId)

	// A credential bound to an organization (API keys always are) is useless
	// without a grant applying to it, for instance after access was revoked.
	if (principalType == "api_key" || credentials.OrganizationId != uuid.Nil) &&
		len(credentials.RoleBindings) == 0 {
		return models.Credentials{}, errors.Errorf("%w: no applicable grant", models.UnAuthorizedError)
	}

	// Likewise, bindings whose caveats are not satisfied by this request (second
	// factor, network, time windows) do not grant access to the organization.
	// Organization access checks rely on the organization of the credentials
	// alone, so they must not be issued without an active binding.
	if credentials.OrganizationId != uuid.Nil &&
		!slices.ContainsFunc(credentials.RoleBindings, func(binding models.RoleBinding) bool {
			return binding.IsActive(credentials.RoleBindingBundle)
		}) {
		return models.Credentials{}, errors.Errorf("%w: no active role binding for this request", models.ForbiddenError)
	}

	credentials.TenantId = tenantId
	credentials.Permissions = models.RoleBindingsPermissions(credentials.RoleBindings, credentials.RoleBindingBundle)

	return credentials, nil
}

func (v *Validator) ValidateTokenOrKey(ctx context.Context, marbleToken, apiKey string, clientIp net.IP) (models.Credentials, error) {
	if apiKey != "" {
		return v.fromAPIKey(ctx, apiKey, clientIp)
	}
	return v.fromMarbleToken(ctx, marbleToken, clientIp)
}

func NewValidator(getter keyAndOrganizationGetter, validator marbleTokenValidator) *Validator {
	return &Validator{
		getter:    getter,
		validator: validator,
	}
}
