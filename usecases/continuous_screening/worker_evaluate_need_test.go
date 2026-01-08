package continuous_screening

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// Tests for CheckIfObjectsNeedScreeningWorker

type CheckIfObjectsNeedScreeningWorkerTestSuite struct {
	suite.Suite
	repository         *mocks.ContinuousScreeningRepository
	clientDbRepository *mocks.ContinuousScreeningClientDbRepository
	taskQueueRepo      *mocks.TaskQueueRepository
	executorFactory    executor_factory.ExecutorFactoryStub
	transactionFactory executor_factory.TransactionFactoryStub

	ctx        context.Context
	orgId      uuid.UUID
	objectType string
}

func (suite *CheckIfObjectsNeedScreeningWorkerTestSuite) SetupTest() {
	suite.repository = new(mocks.ContinuousScreeningRepository)
	suite.clientDbRepository = new(mocks.ContinuousScreeningClientDbRepository)
	suite.taskQueueRepo = new(mocks.TaskQueueRepository)

	suite.executorFactory = executor_factory.NewExecutorFactoryStub()
	suite.transactionFactory = executor_factory.NewTransactionFactoryStub(suite.executorFactory)

	suite.ctx = context.Background()
	suite.orgId = uuid.MustParse("12345678-1234-5678-9012-345678901234")
	suite.objectType = "transactions"
}

func (suite *CheckIfObjectsNeedScreeningWorkerTestSuite) makeWorker() *EvaluateNeedTaskWorker {
	return NewEvaluateNeedTaskWorker(
		suite.executorFactory,
		suite.transactionFactory,
		suite.repository,
		suite.clientDbRepository,
		suite.taskQueueRepo,
	)
}

func (suite *CheckIfObjectsNeedScreeningWorkerTestSuite) AssertExpectations() {
	t := suite.T()
	suite.repository.AssertExpectations(t)
	suite.clientDbRepository.AssertExpectations(t)
	suite.taskQueueRepo.AssertExpectations(t)
}

func TestCheckIfObjectsNeedScreeningWorker(t *testing.T) {
	suite.Run(t, new(CheckIfObjectsNeedScreeningWorkerTestSuite))
}

func (suite *CheckIfObjectsNeedScreeningWorkerTestSuite) TestWork_NoMonitoredObjects_NoEnqueue() {
	// Setup
	objectIds := []string{"object1", "object2"}

	job := &river.Job[models.ContinuousScreeningEvaluateNeedArgs]{
		Args: models.ContinuousScreeningEvaluateNeedArgs{
			OrgId:      suite.orgId,
			ObjectType: suite.objectType,
			ObjectIds:  objectIds,
		},
	}

	// Setup mocks
	suite.repository.On("ListContinuousScreeningConfigByObjectType", suite.ctx, mock.Anything,
		suite.orgId, suite.objectType).Return([]models.ContinuousScreeningConfig{
		{
			Id:          uuid.New(),
			StableId:    uuid.New(),
			ObjectTypes: []string{suite.objectType},
			Enabled:     true,
		},
	}, nil)
	suite.clientDbRepository.On("ListMonitoredObjectsByObjectIds", suite.ctx, mock.Anything,
		suite.objectType, objectIds).Return([]models.ContinuousScreeningMonitoredObject{}, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *CheckIfObjectsNeedScreeningWorkerTestSuite) TestWork_WithMonitoredObjects_EnqueueTasks() {
	// Setup
	objectIds := []string{"object1", "object2"}
	monitoringId1 := uuid.New()
	monitoringId2 := uuid.New()

	monitoredObjects := []models.ContinuousScreeningMonitoredObject{
		{
			Id:       monitoringId1,
			ObjectId: "object1",
		},
		{
			Id:       monitoringId2,
			ObjectId: "object2",
		},
	}

	job := &river.Job[models.ContinuousScreeningEvaluateNeedArgs]{
		Args: models.ContinuousScreeningEvaluateNeedArgs{
			OrgId:      suite.orgId,
			ObjectType: suite.objectType,
			ObjectIds:  objectIds,
		},
	}

	// Setup mocks
	suite.repository.On("ListContinuousScreeningConfigByObjectType", suite.ctx, mock.Anything,
		suite.orgId, suite.objectType).Return([]models.ContinuousScreeningConfig{
		{
			Id:          uuid.New(),
			StableId:    uuid.New(),
			ObjectTypes: []string{suite.objectType},
			Enabled:     true,
		},
	}, nil)
	suite.clientDbRepository.On("ListMonitoredObjectsByObjectIds", suite.ctx, mock.Anything,
		suite.objectType, objectIds).Return(monitoredObjects, nil)
	suite.taskQueueRepo.On("EnqueueContinuousScreeningDoScreeningTaskMany", suite.ctx, mock.Anything,
		suite.orgId, suite.objectType,
		[]uuid.UUID{monitoringId1, monitoringId2},
		models.ContinuousScreeningTriggerTypeObjectUpdated).Return(nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *CheckIfObjectsNeedScreeningWorkerTestSuite) TestWork_NoContinuousScreeningConfig_NoEnqueue() {
	// Setup
	objectIds := []string{"object1", "object2"}

	job := &river.Job[models.ContinuousScreeningEvaluateNeedArgs]{
		Args: models.ContinuousScreeningEvaluateNeedArgs{
			OrgId:      suite.orgId,
			ObjectType: suite.objectType,
			ObjectIds:  objectIds,
		},
	}

	// Setup mocks - ListContinuousScreeningConfigByObjectType returns empty list
	suite.repository.On("ListContinuousScreeningConfigByObjectType", suite.ctx, mock.Anything,
		suite.orgId, suite.objectType).Return([]models.ContinuousScreeningConfig{}, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *CheckIfObjectsNeedScreeningWorkerTestSuite) TestWork_EmptyObjectIds_NoEnqueue() {
	// Setup - empty object IDs
	objectIds := []string{}

	job := &river.Job[models.ContinuousScreeningEvaluateNeedArgs]{
		Args: models.ContinuousScreeningEvaluateNeedArgs{
			OrgId:      suite.orgId,
			ObjectType: suite.objectType,
			ObjectIds:  objectIds,
		},
	}

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}
