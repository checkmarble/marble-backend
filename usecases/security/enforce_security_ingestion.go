package security

import (
	"errors"
	"marble/marble-backend/models"
)

type EnforceSecurityIngestion interface {
	EnforceSecurity
	CanIngest(organizationId string) error

}

type EnforceSecurityIngestionImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityIngestionImpl) CanIngest(organizationId string) error {
	return errors.Join(
		e.Permission(models.INGESTION),
		e.ReadOrganization(organizationId),
	)
}
