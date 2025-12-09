package continuous_screening

import (
	"context"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DoScreeningWorkerTestSuite struct {
	suite.Suite
	repository         *mocks.ContinuousScreeningRepository
	clientDbRepository *mocks.ContinuousScreeningClientDbRepository
	usecase            *mocks.ContinuousScreeningUsecase
	executorFactory    executor_factory.ExecutorFactoryStub
	transactionFactory executor_factory.TransactionFactoryStub

	ctx            context.Context
	configId       uuid.UUID
	configStableId uuid.UUID
	orgId          uuid.UUID
	objectType     string
	objectId       string
	monitoringId   uuid.UUID
}

func (suite *DoScreeningWorkerTestSuite) SetupTest() {
	suite.repository = new(mocks.ContinuousScreeningRepository)
	suite.clientDbRepository = new(mocks.ContinuousScreeningClientDbRepository)
	suite.usecase = new(mocks.ContinuousScreeningUsecase)

	suite.executorFactory = executor_factory.NewExecutorFactoryStub()
	suite.transactionFactory = executor_factory.NewTransactionFactoryStub(suite.executorFactory)

	suite.ctx = context.Background()
	suite.configId = uuid.New()
	suite.configStableId = uuid.New()
	suite.orgId = uuid.MustParse("12345678-1234-1234-1234-123456789012")
	suite.objectType = "transactions"
	suite.objectId = "test-object-id"
	suite.monitoringId = uuid.New()
}

func (suite *DoScreeningWorkerTestSuite) makeWorker() *DoScreeningWorker {
	return NewDoScreeningWorker(
		suite.executorFactory,
		suite.transactionFactory,
		suite.repository,
		suite.clientDbRepository,
		suite.usecase,
	)
}

func (suite *DoScreeningWorkerTestSuite) AssertExpectations() {
	t := suite.T()
	suite.repository.AssertExpectations(t)
	suite.clientDbRepository.AssertExpectations(t)
	suite.usecase.AssertExpectations(t)
}

func TestDoScreeningWorker(t *testing.T) {
	suite.Run(t, new(DoScreeningWorkerTestSuite))
}

func (suite *DoScreeningWorkerTestSuite) TestWork_ObjectUpdated_ScreeningResultUnchanged_SkipCaseCreation() {
	// Setup
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
		Name:        "test-config",
		Description: "test description",
	}

	monitoredObject := models.ContinuousScreeningMonitoredObject{
		Id:             uuid.New(),
		ObjectId:       suite.objectId,
		ConfigStableId: suite.configStableId,
	}

	table := models.Table{
		Name: suite.objectType,
	}

	mapping := models.ContinuousScreeningDataModelMapping{
		Entity:     suite.objectType,
		Properties: map[string]string{},
	}

	ingestedObject := models.DataModelObject{
		Data: map[string]any{
			"id": suite.objectId,
		},
		Metadata: map[string]any{
			"valid_from": time.Now(),
		},
	}

	ingestedObjectInternalId := uuid.New()

	// Same matches as existing screening
	matches := []models.ScreeningMatch{
		{
			EntityId: "entity1",
			Id:       "match1",
			IsMatch:  true,
		},
	}

	screeningWithMatches := models.ScreeningWithMatches{
		Screening: models.Screening{
			Status: models.ScreeningStatusInReview,
		},
		Matches:            matches,
		EffectiveThreshold: 80,
	}

	existingContinuousScreening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id: uuid.New(),
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				OpenSanctionEntityId: "entity1",
			},
		},
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id: uuid.New(),
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				OpenSanctionEntityId: "entity1",
			},
		},
	}

	job := &river.Job[models.ContinuousScreeningDoScreeningArgs]{
		Args: models.ContinuousScreeningDoScreeningArgs{
			ObjectType:   suite.objectType,
			OrgId:        suite.orgId.String(),
			TriggerType:  models.ContinuousScreeningTriggerTypeObjectUpdated,
			MonitoringId: suite.monitoringId,
		},
	}

	// Setup mocks
	suite.clientDbRepository.On("GetMonitoredObject", suite.ctx, mock.Anything,
		suite.monitoringId).Return(monitoredObject, nil)
	suite.repository.On("GetContinuousScreeningConfigByStableId", suite.ctx, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.usecase.On("GetDataModelTableAndMapping", suite.ctx, mock.Anything, config,
		suite.objectType).Return(table, mapping, nil)
	suite.usecase.On("GetIngestedObject", suite.ctx, mock.Anything, table, suite.objectId).Return(
		ingestedObject, ingestedObjectInternalId, nil)
	suite.usecase.On("DoScreening", suite.ctx, mock.Anything, ingestedObject, mapping, config,
		"transactions", "test-object-id").Return(screeningWithMatches, nil)
	suite.repository.On("GetContinuousScreeningByObjectId", suite.ctx, mock.Anything,
		suite.objectId, suite.objectType, suite.orgId, mock.MatchedBy(func(status *models.ScreeningStatus) bool {
			return status == nil
		}), false).Return(&existingContinuousScreening, nil)
	suite.repository.On("GetContinuousScreeningByObjectId", suite.ctx, mock.Anything,
		suite.objectId, suite.objectType, suite.orgId, mock.MatchedBy(func(status *models.ScreeningStatus) bool {
			return status != nil && *status == models.ScreeningStatusInReview
		}), true).Return(&existingContinuousScreening, nil)
	suite.repository.On("InsertContinuousScreening", suite.ctx, mock.Anything,
		screeningWithMatches, config, suite.objectType, suite.objectId, ingestedObjectInternalId,
		models.ContinuousScreeningTriggerTypeObjectUpdated).Return(continuousScreeningWithMatches, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	// Verify that HandleCaseCreation is NOT called because screening result is unchanged
	suite.usecase.AssertNotCalled(suite.T(), "HandleCaseCreation", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
	suite.AssertExpectations()
}

func (suite *DoScreeningWorkerTestSuite) TestWork_ObjectUpdated_ScreeningResultChanged_CallCaseCreation() {
	// Setup
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
		Name:        "test-config",
		Description: "test description",
	}

	monitoredObject := models.ContinuousScreeningMonitoredObject{
		Id:             uuid.New(),
		ObjectId:       suite.objectId,
		ConfigStableId: suite.configStableId,
	}

	table := models.Table{
		Name: suite.objectType,
	}

	mapping := models.ContinuousScreeningDataModelMapping{
		Entity:     suite.objectType,
		Properties: map[string]string{},
	}

	ingestedObject := models.DataModelObject{
		Data: map[string]any{
			"id": suite.objectId,
		},
		Metadata: map[string]any{
			"valid_from": time.Now(),
		},
	}

	ingestedObjectInternalId := uuid.New()

	// Different matches from existing screening
	newMatches := []models.ScreeningMatch{
		{
			EntityId: "entity2", // Different entity
			Id:       "match2",
			IsMatch:  true,
		},
	}

	screeningWithMatches := models.ScreeningWithMatches{
		Screening: models.Screening{
			Status: models.ScreeningStatusInReview,
		},
		Matches:            newMatches,
		EffectiveThreshold: 80,
	}

	existingContinuousScreening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id: uuid.New(),
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				OpenSanctionEntityId: "entity1", // Different from new
			},
		},
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id: uuid.New(),
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				OpenSanctionEntityId: "entity2",
			},
		},
	}

	job := &river.Job[models.ContinuousScreeningDoScreeningArgs]{
		Args: models.ContinuousScreeningDoScreeningArgs{
			ObjectType:   suite.objectType,
			OrgId:        suite.orgId.String(),
			TriggerType:  models.ContinuousScreeningTriggerTypeObjectUpdated,
			MonitoringId: suite.monitoringId,
		},
	}

	// Setup mocks
	suite.clientDbRepository.On("GetMonitoredObject", suite.ctx, mock.Anything,
		suite.monitoringId).Return(monitoredObject, nil)
	suite.repository.On("GetContinuousScreeningConfigByStableId", suite.ctx, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.usecase.On("GetDataModelTableAndMapping", suite.ctx, mock.Anything, config,
		suite.objectType).Return(table, mapping, nil)
	suite.usecase.On("GetIngestedObject", suite.ctx, mock.Anything, table, suite.objectId).Return(
		ingestedObject, ingestedObjectInternalId, nil)
	suite.usecase.On("DoScreening", suite.ctx, mock.Anything, ingestedObject, mapping, config,
		"transactions", "test-object-id").Return(screeningWithMatches, nil)
	suite.repository.On("GetContinuousScreeningByObjectId", suite.ctx, mock.Anything,
		suite.objectId, suite.objectType, suite.orgId, mock.MatchedBy(func(status *models.ScreeningStatus) bool {
			return status == nil
		}), false).Return(&existingContinuousScreening, nil)
	suite.repository.On("GetContinuousScreeningByObjectId", suite.ctx, mock.Anything,
		suite.objectId, suite.objectType, suite.orgId, mock.MatchedBy(func(status *models.ScreeningStatus) bool {
			return status != nil && *status == models.ScreeningStatusInReview
		}), true).Return((*models.ContinuousScreeningWithMatches)(nil), nil)
	suite.repository.On("InsertContinuousScreening", suite.ctx, mock.Anything,
		screeningWithMatches, config, suite.objectType, suite.objectId, ingestedObjectInternalId,
		models.ContinuousScreeningTriggerTypeObjectUpdated).Return(continuousScreeningWithMatches, nil)
	// Return empty case for simplicity because it is not used for this test
	suite.usecase.On("HandleCaseCreation", suite.ctx, mock.Anything, config, suite.objectId,
		continuousScreeningWithMatches).Return(models.Case{}, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *DoScreeningWorkerTestSuite) TestWork_IngestedObjectBeforeLatestScreening_SkipScreening() {
	// Setup
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
		Name:        "test-config",
		Description: "test description",
	}

	monitoredObject := models.ContinuousScreeningMonitoredObject{
		Id:             uuid.New(),
		ObjectId:       suite.objectId,
		ConfigStableId: suite.configStableId,
	}

	table := models.Table{
		Name: suite.objectType,
	}

	mapping := models.ContinuousScreeningDataModelMapping{
		Entity:     suite.objectType,
		Properties: map[string]string{},
	}

	// Ingested object with valid_from timestamp in the past
	pastTime := time.Now().Add(-2 * time.Hour)
	ingestedObject := models.DataModelObject{
		Data: map[string]any{
			"id": suite.objectId,
		},
		Metadata: map[string]any{
			"valid_from": pastTime,
		},
	}

	ingestedObjectInternalId := uuid.New()

	// Existing screening created more recently than the ingested object
	existingContinuousScreening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:        uuid.New(),
			CreatedAt: time.Now(),
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				OpenSanctionEntityId: "entity1",
			},
		},
	}

	job := &river.Job[models.ContinuousScreeningDoScreeningArgs]{
		Args: models.ContinuousScreeningDoScreeningArgs{
			ObjectType:   suite.objectType,
			OrgId:        suite.orgId.String(),
			TriggerType:  models.ContinuousScreeningTriggerTypeObjectUpdated,
			MonitoringId: suite.monitoringId,
		},
	}

	// Setup mocks
	suite.clientDbRepository.On("GetMonitoredObject", suite.ctx, mock.Anything,
		suite.monitoringId).Return(monitoredObject, nil)
	suite.repository.On("GetContinuousScreeningConfigByStableId", suite.ctx, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.usecase.On("GetDataModelTableAndMapping", suite.ctx, mock.Anything, config,
		suite.objectType).Return(table, mapping, nil)
	suite.usecase.On("GetIngestedObject", suite.ctx, mock.Anything, table, suite.objectId).Return(
		ingestedObject, ingestedObjectInternalId, nil)
	// Existing screening is more recent than ingested object
	suite.repository.On("GetContinuousScreeningByObjectId", suite.ctx, mock.Anything,
		suite.objectId, suite.objectType, suite.orgId, mock.MatchedBy(func(status *models.ScreeningStatus) bool {
			return status == nil
		}), false).Return(&existingContinuousScreening, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	// Verify that DoScreening is NOT called because the ingested object is outdated
	suite.usecase.AssertNotCalled(suite.T(), "DoScreening")
	// Verify that InsertContinuousScreening is NOT called
	suite.repository.AssertNotCalled(suite.T(), "InsertContinuousScreening")
	// Verify that GetContinuousScreeningByObjectId is NOT called a second time (with ScreeningStatusInReview filter)
	suite.usecase.AssertNotCalled(suite.T(), "HandleCaseCreation")
	suite.AssertExpectations()
}

func (suite *DoScreeningWorkerTestSuite) TestWork_ObjectAdded_CallCaseCreation() {
	// Setup
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
		Name:        "test-config",
		Description: "test description",
	}

	monitoredObject := models.ContinuousScreeningMonitoredObject{
		Id:             uuid.New(),
		ObjectId:       suite.objectId,
		ConfigStableId: suite.configStableId,
	}

	table := models.Table{
		Name: suite.objectType,
	}

	mapping := models.ContinuousScreeningDataModelMapping{
		Entity:     suite.objectType,
		Properties: map[string]string{},
	}

	ingestedObject := models.DataModelObject{
		Data: map[string]any{
			"id": suite.objectId,
		},
		Metadata: map[string]any{
			"valid_from": time.Now(),
		},
	}

	ingestedObjectInternalId := uuid.New()

	matches := []models.ScreeningMatch{
		{
			EntityId: "entity1",
			Id:       "match1",
			IsMatch:  true,
		},
	}

	screeningWithMatches := models.ScreeningWithMatches{
		Screening: models.Screening{
			Status: models.ScreeningStatusInReview,
		},
		Matches:            matches,
		EffectiveThreshold: 80,
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id: uuid.New(),
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				OpenSanctionEntityId: "entity1",
			},
		},
	}

	job := &river.Job[models.ContinuousScreeningDoScreeningArgs]{
		Args: models.ContinuousScreeningDoScreeningArgs{
			ObjectType:   suite.objectType,
			OrgId:        suite.orgId.String(),
			TriggerType:  models.ContinuousScreeningTriggerTypeObjectAdded,
			MonitoringId: suite.monitoringId,
		},
	}

	// Setup mocks
	suite.clientDbRepository.On("GetMonitoredObject", suite.ctx, mock.Anything,
		suite.monitoringId).Return(monitoredObject, nil)
	suite.repository.On("GetContinuousScreeningConfigByStableId", suite.ctx, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.usecase.On("GetDataModelTableAndMapping", suite.ctx, mock.Anything, config,
		suite.objectType).Return(table, mapping, nil)
	suite.usecase.On("GetIngestedObject", suite.ctx, mock.Anything, table, suite.objectId).Return(
		ingestedObject, ingestedObjectInternalId, nil)
	// For ObjectAdded trigger, there should be no existing screening
	suite.repository.On("GetContinuousScreeningByObjectId", suite.ctx, mock.Anything,
		suite.objectId, suite.objectType, suite.orgId, mock.MatchedBy(func(status *models.ScreeningStatus) bool {
			return status == nil
		}), false).Return(
		(*models.ContinuousScreeningWithMatches)(nil), nil)
	suite.usecase.On("DoScreening", suite.ctx, mock.Anything, ingestedObject, mapping, config,
		"transactions", "test-object-id").Return(screeningWithMatches, nil)
	suite.repository.On("InsertContinuousScreening", suite.ctx, mock.Anything,
		screeningWithMatches, config, suite.objectType, suite.objectId, ingestedObjectInternalId,
		models.ContinuousScreeningTriggerTypeObjectAdded).Return(continuousScreeningWithMatches, nil)
	// Return empty case for simplicity because it is not used for this test
	suite.usecase.On("HandleCaseCreation", suite.ctx, mock.Anything, config, suite.objectId,
		continuousScreeningWithMatches).Return(models.Case{}, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	// For ObjectAdded trigger, the second GetContinuousScreeningByObjectId call is NOT made
	suite.usecase.AssertExpectations(suite.T())
}
