package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
	"github.com/checkmarble/marble-backend/utils"
)

type ScheduledExecutionsTestSuite struct {
	suite.Suite
	exec               *mocks.Executor
	transaction        *mocks.Transaction
	enforceSecurity    *mocks.EnforceSecurity
	transactionFactory *mocks.TransactionFactory
	repository         *mocks.ScheduledExecutionUsecaseRepository
	taskQueue          *mocks.TaskQueueRepository

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
	suite.taskQueue = new(mocks.TaskQueueRepository)

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
		enforceSecurity:     suite.enforceSecurity,
		transactionFactory:  suite.transactionFactory,
		repository:          suite.repository,
		taskQueueRepository: suite.taskQueue,
		manifestBucketUrl:   "file:///tmp/manifests",
	}
}

func (suite *ScheduledExecutionsTestSuite) AssertExpectations() {
	t := suite.T()
	suite.exec.AssertExpectations(t)
	suite.transaction.AssertExpectations(t)
	suite.enforceSecurity.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
	suite.repository.AssertExpectations(t)
	suite.taskQueue.AssertExpectations(t)
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

func (suite *ScheduledExecutionsTestSuite) v2Execution(status models.ScheduledExecutionStatus) models.ScheduledExecution {
	return models.ScheduledExecution{
		Id:              "execution-id",
		OrganizationId:  suite.organizationId,
		Status:          status,
		ManifestBlobKey: utils.Ptr("scheduled_executions/execution-id/manifest.txt"),
	}
}

func (suite *ScheduledExecutionsTestSuite) TestCancelScheduledExecution_processing() {
	ctx := context.Background()
	execution := suite.v2Execution(models.ScheduledExecutionProcessing)

	suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
	suite.repository.On("GetScheduledExecution", suite.transaction, execution.Id).Return(execution, nil)
	suite.enforceSecurity.On("CancelScheduledExecution", execution).Return(nil)

	var deadline time.Time
	suite.repository.On("RequestScheduledExecutionCancel", suite.transaction, execution.Id,
		models.ScheduledExecutionProcessing, mock.AnythingOfType("time.Time")).
		Run(func(args mock.Arguments) { deadline = args.Get(3).(time.Time) }).
		Return(true, nil)
	suite.taskQueue.On("EnqueueScheduledExecutionCancelFinalizer", ctx, suite.transaction,
		suite.organizationId, execution.Id, mock.AnythingOfType("time.Time")).
		Run(func(args mock.Arguments) { assert.Equal(suite.T(), deadline, args.Get(4)) }).
		Return(nil)

	err := suite.makeUsecase().CancelScheduledExecution(ctx, execution.Id)

	t := suite.T()
	assert.NoError(t, err)
	grace := worker_jobs.ScheduledExecutionCancelGracePeriod(models.ScheduledExecutionProcessing)
	assert.WithinDuration(t, time.Now().Add(grace), deadline, 5*time.Second)
	suite.AssertExpectations()
}

func (suite *ScheduledExecutionsTestSuite) TestCancelScheduledExecution_pending_v2() {
	suite.T().Setenv("ENABLE_BATCH_EXECUTION_V2", "all")
	ctx := context.Background()
	execution := suite.v2Execution(models.ScheduledExecutionPending)
	execution.ManifestBlobKey = nil

	suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
	suite.repository.On("GetScheduledExecution", suite.transaction, execution.Id).Return(execution, nil)
	suite.enforceSecurity.On("CancelScheduledExecution", execution).Return(nil)
	suite.repository.On("RequestScheduledExecutionCancel", suite.transaction, execution.Id,
		models.ScheduledExecutionPending, mock.AnythingOfType("time.Time")).Return(true, nil)
	suite.taskQueue.On("EnqueueScheduledExecutionCancelFinalizer", ctx, suite.transaction,
		suite.organizationId, execution.Id, mock.AnythingOfType("time.Time")).Return(nil)

	err := suite.makeUsecase().CancelScheduledExecution(ctx, execution.Id)

	assert.NoError(suite.T(), err)
	suite.AssertExpectations()
}

func (suite *ScheduledExecutionsTestSuite) TestCancelScheduledExecution_v1_is_noop() {
	suite.T().Setenv("ENABLE_BATCH_EXECUTION_V2", "")
	ctx := context.Background()
	pending := suite.v2Execution(models.ScheduledExecutionPending)
	pending.ManifestBlobKey = nil
	processing := suite.v2Execution(models.ScheduledExecutionProcessing)
	processing.ManifestBlobKey = nil

	for _, execution := range []models.ScheduledExecution{pending, processing} {
		suite.SetupTest()
		suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
		suite.repository.On("GetScheduledExecution", suite.transaction, execution.Id).Return(execution, nil)
		suite.enforceSecurity.On("CancelScheduledExecution", execution).Return(nil)

		err := suite.makeUsecase().CancelScheduledExecution(ctx, execution.Id)

		assert.NoError(suite.T(), err)
		suite.AssertExpectations()
	}
}

func (suite *ScheduledExecutionsTestSuite) TestCancelScheduledExecution_already_cancelled() {
	ctx := context.Background()

	for _, status := range []models.ScheduledExecutionStatus{
		models.ScheduledExecutionCancelling, models.ScheduledExecutionCancelled,
	} {
		suite.SetupTest()
		execution := suite.v2Execution(status)
		suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
		suite.repository.On("GetScheduledExecution", suite.transaction, execution.Id).Return(execution, nil)
		suite.enforceSecurity.On("CancelScheduledExecution", execution).Return(nil)

		err := suite.makeUsecase().CancelScheduledExecution(ctx, execution.Id)

		assert.NoError(suite.T(), err)
		suite.AssertExpectations()
	}
}

func (suite *ScheduledExecutionsTestSuite) TestCancelScheduledExecution_finished() {
	ctx := context.Background()

	for _, status := range []models.ScheduledExecutionStatus{
		models.ScheduledExecutionSuccess, models.ScheduledExecutionPartialFailure, models.ScheduledExecutionFailure,
	} {
		suite.SetupTest()
		execution := suite.v2Execution(status)
		suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
		suite.repository.On("GetScheduledExecution", suite.transaction, execution.Id).Return(execution, nil)
		suite.enforceSecurity.On("CancelScheduledExecution", execution).Return(nil)

		err := suite.makeUsecase().CancelScheduledExecution(ctx, execution.Id)

		assert.ErrorIs(suite.T(), err, models.ConflictError)
		suite.AssertExpectations()
	}
}

func (suite *ScheduledExecutionsTestSuite) TestCancelScheduledExecution_security() {
	ctx := context.Background()
	securityError := errors.New("some security error")
	execution := suite.v2Execution(models.ScheduledExecutionProcessing)

	suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
	suite.repository.On("GetScheduledExecution", suite.transaction, execution.Id).Return(execution, nil)
	suite.enforceSecurity.On("CancelScheduledExecution", execution).Return(securityError)

	err := suite.makeUsecase().CancelScheduledExecution(ctx, execution.Id)

	assert.ErrorIs(suite.T(), err, securityError)
	suite.AssertExpectations()
}

func (suite *ScheduledExecutionsTestSuite) TestCancelScheduledExecution_status_changed_concurrently() {
	ctx := context.Background()
	pending := suite.v2Execution(models.ScheduledExecutionPending)
	processing := suite.v2Execution(models.ScheduledExecutionProcessing)
	suite.T().Setenv("ENABLE_BATCH_EXECUTION_V2", "all")

	suite.transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
	suite.repository.On("GetScheduledExecution", suite.transaction, pending.Id).Return(pending, nil).Once()
	suite.repository.On("GetScheduledExecution", suite.transaction, pending.Id).Return(processing, nil).Once()
	suite.enforceSecurity.On("CancelScheduledExecution", mock.Anything).Return(nil)
	suite.repository.On("RequestScheduledExecutionCancel", suite.transaction, pending.Id,
		models.ScheduledExecutionPending, mock.AnythingOfType("time.Time")).Return(false, nil)
	suite.repository.On("RequestScheduledExecutionCancel", suite.transaction, pending.Id,
		models.ScheduledExecutionProcessing, mock.AnythingOfType("time.Time")).Return(true, nil)
	suite.taskQueue.On("EnqueueScheduledExecutionCancelFinalizer", ctx, suite.transaction,
		suite.organizationId, pending.Id, mock.AnythingOfType("time.Time")).Return(nil)

	err := suite.makeUsecase().CancelScheduledExecution(ctx, pending.Id)

	assert.NoError(suite.T(), err)
	suite.AssertExpectations()
}

func TestScheduledExecutions(t *testing.T) {
	suite.Run(t, new(ScheduledExecutionsTestSuite))
}
