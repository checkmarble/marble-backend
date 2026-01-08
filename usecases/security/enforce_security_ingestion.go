package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type EnforceSecurityIngestion interface {
	EnforceSecurity
	CanIngest(organizationId uuid.UUID) error
}

type EnforceSecurityIngestionImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityIngestionImpl) CanIngest(organizationId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.INGESTION),
		e.ReadOrganization(organizationId),
	)
}
