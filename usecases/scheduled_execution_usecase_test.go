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
)

type ScheduledExecutionsTestSuite struct {
	suite.Suite
	exec               *mocks.Executor
	transaction        *mocks.Transaction
	enforceSecurity    *mocks.EnforceSecurity
	transactionFactory *mocks.TransactionFactory
	repository         *mocks.ScheduledExecutionUsecaseRepository

	organizationId      uuid.UUID
	scenarioId          string
	scheduledExecutions []models.ScheduledExecution
}

func (suite *ScheduledExecutionsTestSuite) SetupTest() {
	suite.exec = new(mocks.Executor)
	suite.transaction = new(mocks.Transaction)
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.repository = new(mocks.ScheduledExecutionUsecaseRepository)

	suite.organizationId = uuid.MustParse("12345678-1234-5678-9012-345678901234")
	suite.scenarioId = "some scenario id"
	suite.scheduledExecutions = []models.ScheduledExecution{
		{
			Id: "some ScheduledExecution id",
		},
	}
}

func (suite *ScheduledExecutionsTestSuite) makeUsecase() *ScheduledExecutionUsecase {
	return &ScheduledExecutionUsecase{
		enforceSecurity:    suite.enforceSecurity,
		transactionFactory: suite.transactionFactory,
		repository:         suite.repository,
	}
}

func (suite *ScheduledExecutionsTestSuite) AssertExpectations() {
	t := suite.T()
	suite.exec.AssertExpectations(t)
	suite.transaction.AssertExpectations(t)
	suite.enforceSecurity.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
	suite.repository.AssertExpectations(t)
}

func (suite *ScheduledExecutionsTestSuite) TestListScheduledExecutions_with_OrganizationId() {
	ctx := context.Background()
	suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
	suite.repository.On("ListScheduledExecutions", suite.transaction, models.ListScheduledExecutionsFilters{
		OrganizationId: suite.organizationId,
	}, mock.Anything).Return(suite.scheduledExecutions, nil)
	suite.enforceSecurity.On("ReadScheduledExecution", suite.scheduledExecutions[0]).Return(nil)

	result, err := suite.makeUsecase().ListScheduledExecutions(ctx, suite.organizationId,
		models.ListScheduledExecutionsFilters{}, nil)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, suite.scheduledExecutions, result.Executions)

	suite.AssertExpectations()
}

func (suite *ScheduledExecutionsTestSuite) TestListScheduledExecutions_with_ScenarioId() {
	ctx := context.Background()
	suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
	suite.repository.On("ListScheduledExecutions", suite.transaction, models.ListScheduledExecutionsFilters{
		ScenarioId: suite.scenarioId,
	}, mock.Anything).Return(suite.scheduledExecutions, nil)
	suite.enforceSecurity.On("ReadScheduledExecution", suite.scheduledExecutions[0]).Return(nil)

	result, err := suite.makeUsecase().ListScheduledExecutions(ctx, suite.organizationId, models.ListScheduledExecutionsFilters{
		ScenarioId: suite.scenarioId,
	}, nil)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, suite.scheduledExecutions, result.Executions)

	suite.AssertExpectations()
}

func (suite *ScheduledExecutionsTestSuite) TestListScheduledExecutions_security() {
	ctx := context.Background()
	securityError := errors.New("some security error")

	suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
	suite.repository.On("ListScheduledExecutions", suite.transaction, models.ListScheduledExecutionsFilters{
		ScenarioId: suite.scenarioId,
	}, mock.Anything).Return(suite.scheduledExecutions, nil)
	suite.enforceSecurity.On("ReadScheduledExecution", suite.scheduledExecutions[0]).Return(securityError)

	result, err := suite.makeUsecase().ListScheduledExecutions(ctx, suite.organizationId, models.ListScheduledExecutionsFilters{
		ScenarioId: suite.scenarioId,
	}, nil)

	t := suite.T()
	assert.ErrorIs(t, err, securityError)
	assert.Empty(t, result, suite.scheduledExecutions)

	suite.AssertExpectations()
}

func TestScheduledExecutions(t *testing.T) {
	suite.Run(t, new(ScheduledExecutionsTestSuite))
}
