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

var scenarioId = "some scenario id"
var scheduledExecutions = []models.ScheduledExecution{
	{
		Id: "some ScheduledExecution id",
	},
}

type ScheduledExecutionsTestSuite struct {
	suite.Suite
	transaction                  *mocks.Transaction
	enforceSecurity              *mocks.EnforceSecurity
	transactionFactory           *mocks.TransactionFactory
	scheduledExecutionRepository *mocks.ScheduledExecutionRepository
	exportScheduleExecution      *mocks.ExportDecisionsMock
}

func (suite *ScheduledExecutionsTestSuite) SetupTest() {
	suite.transaction = new(mocks.Transaction)
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.scheduledExecutionRepository = new(mocks.ScheduledExecutionRepository)
	suite.exportScheduleExecution = new(mocks.ExportDecisionsMock)
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
	suite.enforceSecurity.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
	suite.scheduledExecutionRepository.AssertExpectations(t)
	suite.exportScheduleExecution.AssertExpectations(t)
}

func (suite *ScheduledExecutionsTestSuite) TestListScheduledExecutions_ListScheduledExecutions_of_organization() {

	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.scheduledExecutionRepository.On("ListScheduledExecutionsOfOrganization", suite.transaction, "some org id").Return(scheduledExecutions, nil)
	suite.enforceSecurity.On("ReadScheduledExecution", scheduledExecutions[0]).Return(nil)

	result, err := suite.makeUsecase().ListScheduledExecutions("")

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, scheduledExecutions, result)

	suite.AssertExpectations()
}

func (suite *ScheduledExecutionsTestSuite) TestListScheduledExecutions_ListScheduledExecutions_of_scenario() {

	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.scheduledExecutionRepository.On("ListScheduledExecutionsOfScenario", suite.transaction, scenarioId).Return(scheduledExecutions, nil)
	suite.enforceSecurity.On("ReadScheduledExecution", scheduledExecutions[0]).Return(nil)

	result, err := suite.makeUsecase().ListScheduledExecutions(scenarioId)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, scheduledExecutions, result)

	suite.AssertExpectations()
}

func (suite *ScheduledExecutionsTestSuite) TestListScheduledExecutions_ListScheduledExecutions_security() {

	securityError := errors.New("some security error")

	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.scheduledExecutionRepository.On("ListScheduledExecutionsOfScenario", suite.transaction, scenarioId).Return(scheduledExecutions, nil)
	suite.enforceSecurity.On("ReadScheduledExecution", scheduledExecutions[0]).Return(securityError)

	result, err := suite.makeUsecase().ListScheduledExecutions(scenarioId)

	t := suite.T()
	assert.ErrorIs(t, err, securityError)
	assert.Empty(t, result, scheduledExecutions)

	suite.AssertExpectations()
}

func TestScheduledExecutions(t *testing.T) {
	suite.Run(t, new(ScheduledExecutionsTestSuite))
}
