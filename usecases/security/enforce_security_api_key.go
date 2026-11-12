package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityApiKeyImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityApiKeyImpl) ReadApiKey(apiKey models.ApiKey) error {
	return errors.Join(
		e.Permission(models.APIKEY_READ), e.ReadOrganization(apiKey.OrganizationId),
	)
}

func (e *EnforceSecurityApiKeyImpl) CreateApiKey(organizationId uuid.UUID, role models.Role) error {
	if role == models.OIDC_CONFIG_EDITOR {
		if err := e.Permission(models.TENANT_OIDC_CONFIG_UPDATE); err != nil {
			return err
		}
	}
	return errors.Join(
		e.Permission(models.APIKEY_CREATE), e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurityApiKeyImpl) DeleteApiKey(apiKey models.ApiKey) error {
	// For now, we don't have any specific permission for deleting an API key
	return e.CreateApiKey(apiKey.OrganizationId, models.API_CLIENT)
}

func (e *EnforceSecurityApiKeyImpl) CheckApiKeyTenant(tenantID uuid.UUID) error {
	if !e.Credentials.HasRole(models.MARBLE_ADMIN) && (e.Credentials.TenantId == uuid.Nil || e.Credentials.TenantId != tenantID) {
		return models.ForbiddenError
	}
	return nil
}
