package continuous_screening

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ScreeningEnrichmentTestSuite struct {
	suite.Suite
	repository         *mocks.ContinuousScreeningRepository
	screeningProvider  *mocks.OpenSanctionsProvider
	executorFactory    executor_factory.ExecutorFactoryStub
	ctx                context.Context
	screeningId        uuid.UUID
	matchId            uuid.UUID
	entityId           string
}

func (suite *ScreeningEnrichmentTestSuite) SetupTest() {
	suite.repository = new(mocks.ContinuousScreeningRepository)
	suite.screeningProvider = new(mocks.OpenSanctionsProvider)
	suite.executorFactory = executor_factory.NewExecutorFactoryStub()

	suite.ctx = context.Background()
	suite.screeningId = uuid.New()
	suite.matchId = uuid.New()
	suite.entityId = "entity-123"
}

func (suite *ScreeningEnrichmentTestSuite) makeUsecase() *ContinuousScreeningUsecase {
	return &ContinuousScreeningUsecase{
		executorFactory:   suite.executorFactory,
		repository:        suite.repository,
		screeningProvider: suite.screeningProvider,
	}
}

func (suite *ScreeningEnrichmentTestSuite) AssertExpectations() {
	t := suite.T()
	suite.repository.AssertExpectations(t)
	suite.screeningProvider.AssertExpectations(t)
}

func TestScreeningEnrichment(t *testing.T) {
	suite.Run(t, new(ScreeningEnrichmentTestSuite))
}

func (suite *ScreeningEnrichmentTestSuite) TestEnrichContinuousScreeningEntity_Success() {
	// Setup
	uc := suite.makeUsecase()

	originalPayload := []byte(`{"id":"entity-123","name":"Original Name"}`)
	enrichedPayload := []byte(`{"properties":{"fullName":["Enriched Name"]}}`)

	screening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                         suite.screeningId,
			OpenSanctionEntityId:       &suite.entityId,
			OpenSanctionEntityPayload:  originalPayload,
			OpenSanctionEntityEnriched: false,
		},
	}

	suite.repository.On("GetContinuousScreeningWithMatchesById",
		suite.ctx,
		mock.Anything,
		suite.screeningId,
	).Return(screening, nil)

	suite.screeningProvider.On("EnrichMatch",
		suite.ctx,
		mock.MatchedBy(func(m models.ScreeningMatch) bool {
			return m.EntityId == suite.entityId
		}),
	).Return(enrichedPayload, nil)

	suite.repository.On("UpdateContinuousScreeningEntityEnrichedPayload",
		suite.ctx,
		mock.Anything,
		suite.screeningId,
		mock.MatchedBy(func(payload []byte) bool {
			// Should contain merged data
			return len(payload) > 0
		}),
	).Return(nil)

	// Execute
	err := uc.EnrichContinuousScreeningEntityWithoutAuthorization(suite.ctx, suite.screeningId)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScreeningEnrichmentTestSuite) TestEnrichContinuousScreeningEntity_AlreadyEnriched_ReturnsError() {
	// Setup
	uc := suite.makeUsecase()

	screening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                         suite.screeningId,
			OpenSanctionEntityId:       &suite.entityId,
			OpenSanctionEntityPayload:  []byte(`{}`),
			OpenSanctionEntityEnriched: true,
		},
	}

	suite.repository.On("GetContinuousScreeningWithMatchesById",
		suite.ctx,
		mock.Anything,
		suite.screeningId,
	).Return(screening, nil)

	// Execute
	err := uc.EnrichContinuousScreeningEntityWithoutAuthorization(suite.ctx, suite.screeningId)

	// Assert
	suite.Error(err)
	suite.AssertExpectations()
}

func (suite *ScreeningEnrichmentTestSuite) TestEnrichContinuousScreeningEntity_NoEntityId_ReturnsError() {
	// Setup
	uc := suite.makeUsecase()

	screening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                         suite.screeningId,
			OpenSanctionEntityId:       nil, // No entity ID
			OpenSanctionEntityEnriched: false,
		},
	}

	suite.repository.On("GetContinuousScreeningWithMatchesById",
		suite.ctx,
		mock.Anything,
		suite.screeningId,
	).Return(screening, nil)

	// Execute
	err := uc.EnrichContinuousScreeningEntityWithoutAuthorization(suite.ctx, suite.screeningId)

	// Assert
	suite.Error(err)
	suite.AssertExpectations()
}

func (suite *ScreeningEnrichmentTestSuite) TestEnrichContinuousScreeningMatch_Success() {
	// Setup
	uc := suite.makeUsecase()

	originalPayload := []byte(`{"id":"match-1","score":0.8}`)
	enrichedPayload := []byte(`{"properties":{"fullName":["Enriched Name"]}}`)

	match := models.ContinuousScreeningMatch{
		Id:                   suite.matchId,
		OpenSanctionEntityId: suite.entityId,
		Payload:              originalPayload,
		Enriched:             false,
	}

	suite.repository.On("GetContinuousScreeningMatch",
		suite.ctx,
		mock.Anything,
		suite.matchId,
	).Return(match, nil)

	suite.screeningProvider.On("EnrichMatch",
		suite.ctx,
		mock.MatchedBy(func(m models.ScreeningMatch) bool {
			return m.EntityId == suite.entityId
		}),
	).Return(enrichedPayload, nil)

	suite.repository.On("UpdateContinuousScreeningMatchEnrichedPayload",
		suite.ctx,
		mock.Anything,
		suite.matchId,
		mock.MatchedBy(func(payload []byte) bool {
			return len(payload) > 0
		}),
	).Return(nil)

	// Execute
	err := uc.EnrichContinuousScreeningMatchWithoutAuthorization(suite.ctx, suite.matchId)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScreeningEnrichmentTestSuite) TestEnrichContinuousScreeningMatch_AlreadyEnriched_ReturnsError() {
	// Setup
	uc := suite.makeUsecase()

	match := models.ContinuousScreeningMatch{
		Id:                   suite.matchId,
		OpenSanctionEntityId: suite.entityId,
		Payload:              []byte(`{}`),
		Enriched:             true,
	}

	suite.repository.On("GetContinuousScreeningMatch",
		suite.ctx,
		mock.Anything,
		suite.matchId,
	).Return(match, nil)

	// Execute
	err := uc.EnrichContinuousScreeningMatchWithoutAuthorization(suite.ctx, suite.matchId)

	// Assert
	suite.Error(err)
	suite.AssertExpectations()
}

func (suite *ScreeningEnrichmentTestSuite) TestMergePayloads_Success() {
	// Setup
	original := []byte(`{"id":"123","name":"Original"}`)
	new := []byte(`{"properties":{"fullName":["New Name"]},"extra":"data"}`)

	// Execute
	merged, err := mergePayloads(original, new)

	// Assert
	suite.NoError(err)
	suite.NotNil(merged)
	// Merged should contain data from both
	suite.Contains(string(merged), "id")
	suite.Contains(string(merged), "name")
	suite.Contains(string(merged), "properties")
	suite.Contains(string(merged), "extra")
}
