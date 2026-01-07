package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
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
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(
		e.Permission(models.INGESTION),
		e.ReadOrganization(orgId),
	)
}
