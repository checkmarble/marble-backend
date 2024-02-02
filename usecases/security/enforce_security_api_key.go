package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityApiKeyImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityApiKeyImpl) CreateApiKey(organizationId string) error {
	return errors.Join(
		e.Permission(models.APIKEY_CREATE), e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurityApiKeyImpl) DeleteApiKey(apiKey models.ApiKey) error {
	return errors.Join(
		e.Permission(models.APIKEY_DELETE), e.ReadOrganization(apiKey.OrganizationId),
	)
}

func (e *EnforceSecurityApiKeyImpl) ListApiKeys() error {
	return errors.Join(
		e.Permission(models.APIKEY_READ),
	)
}
