package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
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
	orgId, _ := uuid.Parse(scenario.OrganizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScenarioImpl) ReadScenarioIteration(scenarioIteration models.ScenarioIteration) error {
	orgId, _ := uuid.Parse(scenarioIteration.OrganizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScenarioImpl) CreateRule(scenarioIteration models.ScenarioIteration) error {
	orgId, _ := uuid.Parse(scenarioIteration.OrganizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScenarioImpl) ReadScenarioPublication(scenarioPublication models.ScenarioPublication) error {
	orgId, _ := uuid.Parse(scenarioPublication.OrganizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScenarioImpl) PublishScenario(scenario models.Scenario) error {
	orgId, _ := uuid.Parse(scenario.OrganizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_PUBLISH),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScenarioImpl) ListScenarios(organizationId string) error {
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_READ),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScenarioImpl) UpdateScenario(scenario models.Scenario) error {
	orgId, _ := uuid.Parse(scenario.OrganizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScenarioImpl) CreateScenario(organizationId string) error {
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(orgId),
	)
}
