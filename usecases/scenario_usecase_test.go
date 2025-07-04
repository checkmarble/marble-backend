package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

type ScenarioUsecaseTestSuite struct {
	suite.Suite
	exec               *mocks.Executor
	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
	executorFactory    *mocks.ExecutorFactory
	enforceSecurity    *mocks.EnforceSecurity
	scenarioRepository *mocks.ScenarioRepository

	organizationId string
	scenarioId     string
	scenario       models.Scenario
	securityError  error
	ctx            context.Context
}

func (suite *ScenarioUsecaseTestSuite) SetupTest() {
	suite.exec = new(mocks.Executor)
	suite.transaction = new(mocks.Transaction)
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.scenarioRepository = new(mocks.ScenarioRepository)

	suite.securityError = errors.New("some security error")
	suite.organizationId = "25ab6323-1657-4a52-923a-ef6983fe4532"
	suite.scenarioId = "c5968ff7-6142-4623-a6b3-1539f345e5fa"
	suite.scenario = models.Scenario{
		Id:                         suite.scenarioId,
		OrganizationId:             suite.organizationId,
		DecisionToCaseWorkflowType: models.WorkflowDisabled,
	}
	suite.ctx = context.Background()
}

func (suite *ScenarioUsecaseTestSuite) makeUsecase() *ScenarioUsecase {
	return &ScenarioUsecase{
		transactionFactory: suite.transactionFactory,
		executorFactory:    suite.executorFactory,
		enforceSecurity:    suite.enforceSecurity,
		repository:         suite.scenarioRepository,
	}
}

func (suite *ScenarioUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.transaction.AssertExpectations(t)
	suite.enforceSecurity.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
	suite.scenarioRepository.AssertExpectations(t)
}

func (suite *ScenarioUsecaseTestSuite) TestListScenarios() {
	expected := []models.Scenario{suite.scenario}
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.scenarioRepository.On("ListScenariosOfOrganization", suite.transaction,
		suite.organizationId).Return(expected, nil)
	suite.enforceSecurity.On("ReadScenario", suite.scenario).Return(nil)

	result, err := suite.makeUsecase().ListScenarios(suite.ctx, suite.organizationId)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	suite.AssertExpectations()
}

func (suite *ScenarioUsecaseTestSuite) TestListScenarios_security() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.scenarioRepository.On("ListScenariosOfOrganization", suite.transaction,
		suite.organizationId).Return([]models.Scenario{suite.scenario}, nil)
	suite.enforceSecurity.On("ReadScenario", suite.scenario).Return(suite.securityError)

	_, err := suite.makeUsecase().ListScenarios(suite.ctx, suite.organizationId)

	assert.ErrorIs(suite.T(), err, suite.securityError)
	suite.AssertExpectations()
}

func (suite *ScenarioUsecaseTestSuite) TestGetScenario() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.scenarioRepository.On("GetScenarioById", suite.transaction, suite.scenarioId).Return(suite.scenario, nil)
	suite.enforceSecurity.On("ReadScenario", suite.scenario).Return(nil)

	result, err := suite.makeUsecase().GetScenario(suite.ctx, suite.scenarioId)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, suite.scenario, result)

	suite.AssertExpectations()
}

func (suite *ScenarioUsecaseTestSuite) TestGetScenario_security() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.scenarioRepository.On("GetScenarioById", suite.transaction, suite.scenarioId).Return(suite.scenario, nil)
	suite.enforceSecurity.On("ReadScenario", suite.scenario).Return(suite.securityError)

	_, err := suite.makeUsecase().GetScenario(suite.ctx, suite.scenarioId)

	assert.ErrorIs(suite.T(), err, suite.securityError)
	suite.AssertExpectations()
}

func (suite *ScenarioUsecaseTestSuite) TestUpdateScenario() {
	scenarioInput := models.UpdateScenarioInput{
		Id: suite.scenarioId,
	}

	updatedScenario := models.Scenario{
		Id:   suite.scenarioId,
		Name: "updated scenario",
	}

	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.scenarioRepository.On("GetScenarioById", suite.transaction, suite.scenarioId).Return(suite.scenario, nil).Once()
	suite.enforceSecurity.On("UpdateScenario", suite.scenario).Return(nil)

	suite.scenarioRepository.On("UpdateScenario", suite.transaction, scenarioInput).Return(nil)
	suite.scenarioRepository.On("GetScenarioById", suite.transaction, suite.scenarioId).Return(updatedScenario, nil).Once()

	result, err := suite.makeUsecase().UpdateScenario(suite.ctx, scenarioInput)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, updatedScenario, result)

	suite.AssertExpectations()
}

func (suite *ScenarioUsecaseTestSuite) TestUpdateScenario_with_workflow() {
	aUuid := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	scenarioInput := models.UpdateScenarioInput{
		Id:                    suite.scenarioId,
		DecisionToCaseInboxId: pure_utils.NullFrom(aUuid),
	}

	scenario := suite.scenario
	scenario.DecisionToCaseWorkflowType = models.WorkflowCreateCase
	scenario.DecisionToCaseInboxId = utils.Ptr(aUuid)
	scenario.DecisionToCaseOutcomes = []models.Outcome{models.Decline}

	updatedScenario := scenario
	updatedScenario.DecisionToCaseInboxId = utils.Ptr(aUuid)

	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.scenarioRepository.On("GetScenarioById", suite.transaction, suite.scenarioId).Return(scenario, nil).Once()
	suite.enforceSecurity.On("UpdateScenario", scenario).Return(nil)
	suite.enforceSecurity.On("PublishScenario", scenario).Return(nil)

	suite.scenarioRepository.On("UpdateScenario", suite.transaction, scenarioInput).Return(nil)
	suite.scenarioRepository.On("GetScenarioById", suite.transaction, suite.scenarioId).Return(updatedScenario, nil).Once()

	result, err := suite.makeUsecase().UpdateScenario(suite.ctx, scenarioInput)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, updatedScenario, result)

	suite.AssertExpectations()
}

func (suite *ScenarioUsecaseTestSuite) TestUpdateScenario_with_workflow_error() {
	aUuid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	scenarioInput := models.UpdateScenarioInput{
		Id:                    suite.scenarioId,
		DecisionToCaseInboxId: pure_utils.Null[uuid.UUID]{Valid: false, Set: true},
	}

	scenario := suite.scenario
	scenario.DecisionToCaseWorkflowType = models.WorkflowCreateCase
	scenario.DecisionToCaseInboxId = utils.Ptr(aUuid)
	scenario.DecisionToCaseOutcomes = []models.Outcome{models.Decline}

	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.scenarioRepository.On("GetScenarioById", suite.transaction, suite.scenarioId).Return(scenario, nil).Once()
	suite.enforceSecurity.On("UpdateScenario", scenario).Return(nil)
	suite.enforceSecurity.On("PublishScenario", scenario).Return(nil)

	_, err := suite.makeUsecase().UpdateScenario(suite.ctx, scenarioInput)

	t := suite.T()
	assert.ErrorIs(t, err, models.BadParameterError)

	suite.AssertExpectations()
}

func (suite *ScenarioUsecaseTestSuite) TestUpdateScenario_security() {
	scenarioInput := models.UpdateScenarioInput{
		Id: suite.scenarioId,
	}

	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.scenarioRepository.On("GetScenarioById", suite.transaction, suite.scenarioId).Return(suite.scenario, nil).Once()
	suite.enforceSecurity.On("UpdateScenario", suite.scenario).Return(suite.securityError)

	_, err := suite.makeUsecase().UpdateScenario(suite.ctx, scenarioInput)

	assert.ErrorIs(suite.T(), err, suite.securityError)
	suite.AssertExpectations()
}

func (suite *ScenarioUsecaseTestSuite) TestCreateScenario() {
	createScenarioInput := models.CreateScenarioInput{
		Name:           "new scenario",
		OrganizationId: suite.organizationId,
	}

	suite.enforceSecurity.On("CreateScenario", suite.organizationId).Return(nil)

	suite.scenarioRepository.On("CreateScenario", suite.transaction, suite.organizationId,
		createScenarioInput, mock.Anything).Return(nil)
	suite.scenarioRepository.On("GetScenarioById", suite.transaction, mock.Anything).Return(suite.scenario, nil).Once()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	result, err := suite.makeUsecase().CreateScenario(suite.ctx, createScenarioInput)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, suite.scenario, result)

	suite.AssertExpectations()
}

func (suite *ScenarioUsecaseTestSuite) TestCreateScenario_security() {
	suite.enforceSecurity.On("CreateScenario", suite.organizationId).Return(suite.securityError)

	_, err := suite.makeUsecase().CreateScenario(context.Background(), models.CreateScenarioInput{
		OrganizationId: suite.organizationId,
	})
	assert.ErrorIs(suite.T(), err, suite.securityError)

	suite.AssertExpectations()
}

func TestScenarioUsecase(t *testing.T) {
	suite.Run(t, new(ScenarioUsecaseTestSuite))
}
