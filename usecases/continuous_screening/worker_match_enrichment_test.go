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

type MatchEnrichmentWorkerTestSuite struct {
	suite.Suite
	repository            *mocks.ContinuousScreeningRepository
	openSanctionsProvider *mocks.OpenSanctionsRepository
	executorFactory       executor_factory.ExecutorFactoryStub
	ctx                   context.Context
	continuousScreeningId uuid.UUID
	orgId                 uuid.UUID
}

func (suite *MatchEnrichmentWorkerTestSuite) SetupTest() {
	suite.repository = new(mocks.ContinuousScreeningRepository)
	suite.openSanctionsProvider = new(mocks.OpenSanctionsRepository)
	suite.executorFactory = executor_factory.NewExecutorFactoryStub()

	suite.ctx = context.Background()
	suite.continuousScreeningId = uuid.New()
	suite.orgId = uuid.MustParse("12345678-1234-1234-1234-123456789012")
}

func (suite *MatchEnrichmentWorkerTestSuite) makeWorker() *ContinuousScreeningMatchEnrichmentWorker {
	return NewContinuousScreeningMatchEnrichmentWorker(
		suite.executorFactory,
		suite.openSanctionsProvider,
		suite.repository,
	)
}

func (suite *MatchEnrichmentWorkerTestSuite) AssertExpectations() {
	t := suite.T()
	suite.repository.AssertExpectations(t)
	suite.openSanctionsProvider.AssertExpectations(t)
}

func TestMatchEnrichmentWorker(t *testing.T) {
	suite.Run(t, new(MatchEnrichmentWorkerTestSuite))
}

func (suite *MatchEnrichmentWorkerTestSuite) TestWork_OpenSanctionsNotConfigured_Aborts() {
	// Setup
	worker := suite.makeWorker()
	job := &river.Job[models.ContinuousScreeningMatchEnrichmentArgs]{
		Args: models.ContinuousScreeningMatchEnrichmentArgs{
			ContinuousScreeningId: suite.continuousScreeningId,
		},
	}

	suite.openSanctionsProvider.On("IsConfigured", suite.ctx).Return(false, nil)

	// Execute
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *MatchEnrichmentWorkerTestSuite) TestWork_OpenSanctionsNotSelfHosted_Aborts() {
	// Setup
	worker := suite.makeWorker()
	job := &river.Job[models.ContinuousScreeningMatchEnrichmentArgs]{
		Args: models.ContinuousScreeningMatchEnrichmentArgs{
			ContinuousScreeningId: suite.continuousScreeningId,
		},
	}

	suite.openSanctionsProvider.On("IsConfigured", suite.ctx).Return(true, nil)
	suite.openSanctionsProvider.On("IsSelfHosted", suite.ctx).Return(false)

	// Execute
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *MatchEnrichmentWorkerTestSuite) TestWork_DatasetTriggered_EnrichesOnlyEntity() {
	// Setup
	worker := suite.makeWorker()
	job := &river.Job[models.ContinuousScreeningMatchEnrichmentArgs]{
		Args: models.ContinuousScreeningMatchEnrichmentArgs{
			ContinuousScreeningId: suite.continuousScreeningId,
		},
	}

	entityId := "entity-123"
	match1Id := uuid.New()
	match2Id := uuid.New()

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                         suite.continuousScreeningId,
			OrgId:                      suite.orgId,
			OpenSanctionEntityId:       &entityId,
			OpenSanctionEntityPayload:  []byte(`{"id":"entity-123"}`),
			OpenSanctionEntityEnriched: false,
			TriggerType:                models.ContinuousScreeningTriggerTypeDatasetUpdated,
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				Id:                   match1Id,
				OpenSanctionEntityId: "match-1",
				Payload:              []byte(`{"id":"match-1"}`),
				Enriched:             false,
			},
			{
				Id:                   match2Id,
				OpenSanctionEntityId: "match-2",
				Payload:              []byte(`{"id":"match-2"}`),
				Enriched:             true, // Already enriched
			},
		},
	}

	suite.openSanctionsProvider.On("IsConfigured", suite.ctx).Return(true, nil)
	suite.openSanctionsProvider.On("IsSelfHosted", suite.ctx).Return(true)
	suite.repository.On("GetContinuousScreeningWithMatchesById",
		suite.ctx,
		mock.Anything,
		suite.continuousScreeningId,
	).Return(continuousScreeningWithMatches, nil)

	// Expect only entity enrichment (not matches, as they are organization's own data)
	enrichedPayload := []byte(`{"id":"entity-123","enriched":true}`)
	suite.openSanctionsProvider.On("EnrichMatch", suite.ctx, models.ScreeningMatch{
		EntityId: entityId,
	}).Return(enrichedPayload, nil)
	suite.repository.On("UpdateContinuousScreeningEntityEnrichedPayload",
		suite.ctx,
		mock.Anything,
		suite.continuousScreeningId,
		mock.Anything,
	).Return(nil)

	// Execute
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *MatchEnrichmentWorkerTestSuite) TestWork_ObjectTriggered_EnrichesOnlyMatches() {
	// Setup
	worker := suite.makeWorker()
	job := &river.Job[models.ContinuousScreeningMatchEnrichmentArgs]{
		Args: models.ContinuousScreeningMatchEnrichmentArgs{
			ContinuousScreeningId: suite.continuousScreeningId,
		},
	}

	match1Id := uuid.New()

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:          suite.continuousScreeningId,
			OrgId:       suite.orgId,
			TriggerType: models.ContinuousScreeningTriggerTypeObjectUpdated,
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				Id:                   match1Id,
				OpenSanctionEntityId: "match-1",
				Payload:              []byte(`{"id":"match-1"}`),
				Enriched:             false,
			},
		},
	}

	suite.openSanctionsProvider.On("IsConfigured", suite.ctx).Return(true, nil)
	suite.openSanctionsProvider.On("IsSelfHosted", suite.ctx).Return(true)
	suite.repository.On("GetContinuousScreeningWithMatchesById",
		suite.ctx,
		mock.Anything,
		suite.continuousScreeningId,
	).Return(continuousScreeningWithMatches, nil)

	// Only expect match enrichment (no entity enrichment for ObjectTriggered)
	enrichedMatchPayload := []byte(`{"id":"match-1","enriched":true}`)
	suite.openSanctionsProvider.On("EnrichMatch", suite.ctx, models.ScreeningMatch{
		EntityId: "match-1",
	}).Return(enrichedMatchPayload, nil)
	suite.repository.On("UpdateContinuousScreeningMatchEnrichedPayload",
		suite.ctx,
		mock.Anything,
		match1Id,
		mock.Anything,
	).Return(nil)

	// Execute
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *MatchEnrichmentWorkerTestSuite) TestTimeout() {
	worker := suite.makeWorker()
	job := &river.Job[models.ContinuousScreeningMatchEnrichmentArgs]{}

	timeout := worker.Timeout(job)

	suite.Equal(10*time.Second, timeout)
}
