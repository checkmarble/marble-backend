package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityScenario interface {
	EnforceSecurity
	ReadScenario(scenario models.Scenario) error
	ReadScenarioIteration(scenarioIteration models.ScenarioIteration) error
	ReadScenarioPublication(scenarioPublication models.ScenarioPublication) error
	PublishScenario(scenario models.Scenario) error
	UpdateScenario(scenario models.Scenario) error
	ListScenarios(organizationId string) error
	CreateScenario(organizationId string) error
	CreateRule(scenarioIteration models.ScenarioIteration) error
}

type EnforceSecurityScenarioImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityScenarioImpl) ReadScenario(scenario models.Scenario) error {

	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(scenario.OrganizationId),
	)
}

func (e *EnforceSecurityScenarioImpl) ReadScenarioIteration(scenarioIteration models.ScenarioIteration) error {
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(scenarioIteration.OrganizationId),
	)
}

func (e *EnforceSecurityScenarioImpl) CreateRule(scenarioIteration models.ScenarioIteration) error {
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(scenarioIteration.OrganizationId),
	)
}

func (e *EnforceSecurityScenarioImpl) ReadScenarioPublication(scenarioPublication models.ScenarioPublication) error {
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(scenarioPublication.OrganizationId),
	)
}

func (e *EnforceSecurityScenarioImpl) PublishScenario(scenario models.Scenario) error {
	return errors.Join(
		e.Permission(models.SCENARIO_PUBLISH),
		e.ReadOrganization(scenario.OrganizationId),
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
		e.ReadOrganization(scenario.OrganizationId),
	)
}

func (e *EnforceSecurityScenarioImpl) CreateScenario(organizationId string) error {
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(organizationId),
	)
}
