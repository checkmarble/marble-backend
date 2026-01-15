package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityTestRun interface {
	EnforceSecurity
	CreateTestRun(organizationId uuid.UUID) error
	ListTestRuns(organizationId uuid.UUID) error
	ReadTestRun(organizationId uuid.UUID) error
}

type EnforceSecurotyTestRunImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurotyTestRunImpl) CreateTestRun(organizationId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurotyTestRunImpl) ListTestRuns(organizationId uuid.UUID) error {
	if e.Credentials.Role == models.MARBLE_ADMIN {
		return errors.Join(
			e.Permission(models.SCENARIO_READ),
		)
	}
	if organizationId == uuid.Nil {
		return errors.Wrap(models.ForbiddenError, "non-admin cannot list scenarios without organization_id")
	}
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurotyTestRunImpl) ReadTestRun(organizationId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(organizationId),
	)
}
