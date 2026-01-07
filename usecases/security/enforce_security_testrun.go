package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityTestRun interface {
	EnforceSecurity
	CreateTestRun(organizationId string) error
	ListTestRuns(organizationId string) error
	ReadTestRun(organizationId string) error
}

type EnforceSecurotyTestRunImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurotyTestRunImpl) CreateTestRun(organizationId string) error {
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurotyTestRunImpl) ListTestRuns(organizationId string) error {
	if e.Credentials.Role == models.MARBLE_ADMIN {
		return errors.Join(
			e.Permission(models.SCENARIO_READ),
		)
	}
	if organizationId == "" {
		return errors.Wrap(models.ForbiddenError, "non-admin cannot list scenarios without organization_id")
	}
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurotyTestRunImpl) ReadTestRun(organizationId string) error {
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(orgId),
	)
}
