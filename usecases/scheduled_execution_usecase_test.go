package usecases

import (
	"errors"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ScheduledExecutionsTestSuite struct {
	suite.Suite
	transaction                  *mocks.Transaction
	enforceSecurity              *mocks.EnforceSecurity
	transactionFactory           *mocks.TransactionFactory
	scheduledExecutionRepository *mocks.ScheduledExecutionRepository
	exportScheduleExecution      *mocks.ExportDecisionsMock

	scenarioId          string
	scheduledExecutions []models.ScheduledExecution
}

func (suite *ScheduledExecutionsTestSuite) SetupTest() {

	suite.transaction = new(mocks.Transaction)
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.scheduledExecutionRepository = new(mocks.ScheduledExecutionRepository)
	suite.exportScheduleExecution = new(mocks.ExportDecisionsMock)

	suite.scenarioId = "some scenario id"
	suite.scheduledExecutions = []models.ScheduledExecution{
		{
			Id: "some ScheduledExecution id",
		},
	}
}

func (suite *ScheduledExecutionsTestSuite) makeUsecase() *ScheduledExecutionUsecase {
	return &ScheduledExecutionUsecase{
		enforceSecurity:              suite.enforceSecurity,
		transactionFactory:           suite.transactionFactory,
		scheduledExecutionRepository: suite.scheduledExecutionRepository,
		exportScheduleExecution:      suite.exportScheduleExecution,
		organizationIdOfContext: func() (string, error) {
			return "some org id", nil
		},
	}
}

func (suite *ScheduledExecutionsTestSuite) AssertExpectations() {
	t := suite.T()
	suite.transaction.AssertExpectations(t)
	suite.enforceSecurity.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
	suite.scheduledExecutionRepository.AssertExpectations(t)
	suite.exportScheduleExecution.AssertExpectations(t)
}

func (suite *ScheduledExecutionsTestSuite) TestListScheduledExecutions_with_OrganizationId() {

	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.scheduledExecutionRepository.On("ListScheduledExecutions", suite.transaction, models.ListScheduledExecutionsFilters{OrganizationId: "some org id"}).Return(suite.scheduledExecutions, nil)
	suite.enforceSecurity.On("ReadScheduledExecution", suite.scheduledExecutions[0]).Return(nil)

	result, err := suite.makeUsecase().ListScheduledExecutions("")

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, suite.scheduledExecutions, result)

	suite.AssertExpectations()
}

func (suite *ScheduledExecutionsTestSuite) TestListScheduledExecutions_with_ScenarioId() {

	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.scheduledExecutionRepository.On("ListScheduledExecutions", suite.transaction, models.ListScheduledExecutionsFilters{ScenarioId: suite.scenarioId}).Return(suite.scheduledExecutions, nil)
	suite.enforceSecurity.On("ReadScheduledExecution", suite.scheduledExecutions[0]).Return(nil)

	result, err := suite.makeUsecase().ListScheduledExecutions(suite.scenarioId)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, suite.scheduledExecutions, result)

	suite.AssertExpectations()
}

func (suite *ScheduledExecutionsTestSuite) TestListScheduledExecutions_security() {

	securityError := errors.New("some security error")

	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.scheduledExecutionRepository.On("ListScheduledExecutions", suite.transaction, models.ListScheduledExecutionsFilters{ScenarioId: suite.scenarioId}).Return(suite.scheduledExecutions, nil)
	suite.enforceSecurity.On("ReadScheduledExecution", suite.scheduledExecutions[0]).Return(securityError)

	result, err := suite.makeUsecase().ListScheduledExecutions(suite.scenarioId)

	t := suite.T()
	assert.ErrorIs(t, err, securityError)
	assert.Empty(t, result, suite.scheduledExecutions)

	suite.AssertExpectations()
}

func TestScheduledExecutions(t *testing.T) {
	suite.Run(t, new(ScheduledExecutionsTestSuite))
}
