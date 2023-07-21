package security

import (
	"errors"
	"marble/marble-backend/models"
)

type EnforceSecurityScenario interface {
	EnforceSecurity
	ReadScenario(scenario models.Scenario) error
	ReadScenarioPublication(scenarioPublication models.ScenarioPublication) error
	PublishScenario(scenario models.Scenario) error
	UpdateScenario(scenario models.Scenario) error
	ListScenarios(organizationId string) error
	CreateScenario(organizationId string) error
}

type EnforceSecurityScenarioImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityScenarioImpl) ReadScenario(scenario models.Scenario) error {

	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(scenario.OrganizationID),
	)
}

func (e *EnforceSecurityScenarioImpl) ReadScenarioPublication(scenarioPublication models.ScenarioPublication) error {
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(scenarioPublication.OrgID),
	)
}

func (e *EnforceSecurityScenarioImpl) PublishScenario(scenario models.Scenario) error {
	return errors.Join(
		e.Permission(models.SCENARIO_PUBLISH),
		e.ReadOrganization(scenario.OrganizationID),
	)
}

func (e *EnforceSecurityScenarioImpl) ListScenarios(organizationId string) error {
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurityScenarioImpl) UpdateScenario(scenario models.Scenario) error {
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(scenario.OrganizationID),
	)
}

func (e *EnforceSecurityScenarioImpl) CreateScenario(organizationId string) error {
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(organizationId),
	)
}
