package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
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
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurotyTestRunImpl) ListTestRuns(organizationId string) error {
	if e.Credentials.Role == models.MARBLE_ADMIN {
		return errors.Join(
			e.Permission(models.SCENARIO_LIST),
		)
	}
	if organizationId == "" {
		return errors.Wrap(models.ForbiddenError, "non-admin cannot list scenarios without organization_id")
	}
	return errors.Join(
		e.Permission(models.SCENARIO_LIST),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurotyTestRunImpl) ReadTestRun(organizationId string) error {
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(organizationId),
	)
}
