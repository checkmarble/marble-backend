package security

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
)

type EnforceSecurity interface {
	Permission(permission models.Permission) error
	ReadOrganization(organizationId string) error
	ReadScenario(scenario models.Scenario) error
	ListScenarios(organizationId string) error
}

type EnforceSecurityImpl struct {
	Credentials models.Credentials
}

func (e *EnforceSecurityImpl) ReadOrganization(organizationId string) error {
	return utils.EnforceOrganizationAccess(e.Credentials, organizationId)
}

func (e *EnforceSecurityImpl) ReadScenario(scenario models.Scenario) error {

	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(scenario.OrganizationID),
	)
}

func (e *EnforceSecurityImpl) ListScenarios(organizationId string) error {
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurityImpl) Permission(permission models.Permission) error {
	if !e.Credentials.Role.HasPermission(permission) {
		return fmt.Errorf("missing permission %s %w", permission.String(), models.ForbiddenError)
	}
	return nil
}
