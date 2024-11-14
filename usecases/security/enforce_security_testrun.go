package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityTestRun interface {
	EnforceSecurity
	CreateTestRun(organizationId string) error
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