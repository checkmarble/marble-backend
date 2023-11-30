package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurity struct {
	mock.Mock
}

func (e *EnforceSecurity) ReadOrganization(organizationId string) error {
	args := e.Called(organizationId)
	return args.Error(0)
}

func (e *EnforceSecurity) Permission(permission models.Permission) error {
	args := e.Called(permission)
	return args.Error(0)
}

func (e *EnforceSecurity) ReadDecision(decision models.Decision) error {
	args := e.Called(decision)
	return args.Error(0)
}

func (e *EnforceSecurity) ReadScheduledExecution(scheduledExecution models.ScheduledExecution) error {
	args := e.Called(scheduledExecution)
	return args.Error(0)
}

func (e *EnforceSecurity) CreateDecision(organizationId string) error {
	args := e.Called(organizationId)
	return args.Error(0)
}

func (e *EnforceSecurity) CreateScheduledExecution(organizationId string) error {
	args := e.Called(organizationId)
	return args.Error(0)
}

func (e *EnforceSecurity) ReadScenario(scenario models.Scenario) error {
	args := e.Called(scenario)
	return args.Error(0)
}

func (e *EnforceSecurity) ReadScenarioIteration(scenarioIteration models.ScenarioIteration) error {
	args := e.Called(scenarioIteration)
	return args.Error(0)
}

func (e *EnforceSecurity) ReadScenarioPublication(scenarioPublication models.ScenarioPublication) error {
	args := e.Called(scenarioPublication)
	return args.Error(0)
}

func (e *EnforceSecurity) PublishScenario(scenario models.Scenario) error {
	args := e.Called(scenario)
	return args.Error(0)
}

func (e *EnforceSecurity) UpdateScenario(scenario models.Scenario) error {
	args := e.Called(scenario)
	return args.Error(0)
}

func (e *EnforceSecurity) ListScenarios(organizationId string) error {
	args := e.Called(organizationId)
	return args.Error(0)
}

func (e *EnforceSecurity) CreateScenario(organizationId string) error {
	args := e.Called(organizationId)
	return args.Error(0)
}

func (e *EnforceSecurity) CreateRule(scenarioIteration models.ScenarioIteration) error {
	args := e.Called(scenarioIteration)
	return args.Error(0)
}

func (e *EnforceSecurity) ReadInbox(i models.Inbox) error {
	args := e.Called(i)
	return args.Error(0)
}

func (e *EnforceSecurity) CreateInbox(organizationId string) error {
	args := e.Called(organizationId)
	return args.Error(0)
}

func (e *EnforceSecurity) ReadInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error {
	args := e.Called(inboxUser, actorInboxUsers)
	return args.Error(0)
}

func (e *EnforceSecurity) CreateInboxUser(i models.CreateInboxUserInput, actorInboxUsers []models.InboxUser, targetInbox models.Inbox, targetUser models.User) error {
	args := e.Called(i, actorInboxUsers, targetInbox, targetUser)
	return args.Error(0)
}
