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

type ScreeningTestSuite struct {
	suite.Suite
	enforceSecurity              *mocks.EnforceSecurity
	repository                   *mocks.ContinuousScreeningRepository
	clientDbRepository           *mocks.ContinuousScreeningClientDbRepository
	organizationSchemaRepository *mocks.OrganizationSchemaRepository
	ingestedDataReader           *mocks.ContinuousScreeningIngestedDataReader
	ingestionUsecase             *mocks.ContinuousScreeningIngestionUsecase
	screeningProvider            *mocks.ContinuousScreeningScreeningProvider
	caseEditor                   *mocks.CaseEditor
	executorFactory              executor_factory.ExecutorFactoryStub
	transactionFactory           executor_factory.TransactionFactoryStub

	ctx         context.Context
	orgId       uuid.UUID
	caseId      uuid.UUID
	matchId     uuid.UUID
	screeningId uuid.UUID
	userId      models.UserId
}

func (suite *ScreeningTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.repository = new(mocks.ContinuousScreeningRepository)
	suite.clientDbRepository = new(mocks.ContinuousScreeningClientDbRepository)
	suite.organizationSchemaRepository = new(mocks.OrganizationSchemaRepository)
	suite.ingestedDataReader = new(mocks.ContinuousScreeningIngestedDataReader)
	suite.ingestionUsecase = new(mocks.ContinuousScreeningIngestionUsecase)
	suite.screeningProvider = new(mocks.ContinuousScreeningScreeningProvider)
	suite.caseEditor = new(mocks.CaseEditor)

	suite.executorFactory = executor_factory.NewExecutorFactoryStub()
	suite.transactionFactory = executor_factory.NewTransactionFactoryStub(suite.executorFactory)

	suite.ctx = context.Background()
	suite.orgId = uuid.MustParse("12345678-1234-1234-1234-123456789012")
	suite.caseId = uuid.New()
	suite.matchId = uuid.New()
	suite.screeningId = uuid.New()
	suite.userId = models.UserId("12345678-1234-1234-1234-123456789012")
}

func (suite *ScreeningTestSuite) makeUsecase() *ContinuousScreeningUsecase {
	return &ContinuousScreeningUsecase{
		executorFactory:              suite.executorFactory,
		transactionFactory:           suite.transactionFactory,
		enforceSecurity:              suite.enforceSecurity,
		enforceSecurityCase:          suite.enforceSecurity,
		enforceSecurityScreening:     suite.enforceSecurity,
		repository:                   suite.repository,
		clientDbRepository:           suite.clientDbRepository,
		organizationSchemaRepository: suite.organizationSchemaRepository,
		ingestedDataReader:           suite.ingestedDataReader,
		ingestionUsecase:             suite.ingestionUsecase,
		screeningProvider:            suite.screeningProvider,
		caseEditor:                   suite.caseEditor,
		inboxReader:                  suite.repository,
	}
}

func (suite *ScreeningTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.repository.AssertExpectations(t)
	suite.clientDbRepository.AssertExpectations(t)
	suite.organizationSchemaRepository.AssertExpectations(t)
	suite.ingestedDataReader.AssertExpectations(t)
	suite.ingestionUsecase.AssertExpectations(t)
	suite.screeningProvider.AssertExpectations(t)
	suite.caseEditor.AssertExpectations(t)
}

func TestScreeningTestSuite(t *testing.T) {
	suite.Run(t, new(ScreeningTestSuite))
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_NotAttachedToCase() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusConfirmedHit,
		ReviewerId: &suite.userId,
	}

	continuousScreeningMatch := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:        suite.screeningId,
			OrgId:     suite.orgId,
			Status:    models.ScreeningStatusInReview,
			CaseId:    nil, // Not attached to a case
			IsPartial: false,
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch},
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningMatch", mock.Anything, mock.Anything,
		suite.matchId).Return(continuousScreeningMatch, nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "continuous screening is not in case")
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_ConfirmedHit_WithMultipleMatches() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusConfirmedHit,
		ReviewerId: &suite.userId,
	}

	continuousScreeningMatch1 := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id-1",
	}
	continuousScreeningMatch2 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id-2",
	}
	continuousScreeningMatch3 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id-3",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:        suite.screeningId,
			OrgId:     suite.orgId,
			Status:    models.ScreeningStatusInReview,
			CaseId:    &suite.caseId,
			IsPartial: false,
		},
		Matches: []models.ContinuousScreeningMatch{
			continuousScreeningMatch1,
			continuousScreeningMatch2, continuousScreeningMatch3,
		},
	}

	caseData := models.Case{
		Id: suite.caseId.String(),
	}

	updatedMatch := continuousScreeningMatch1
	updatedMatch.Status = models.ScreeningMatchStatusConfirmedHit

	// Mock expectations
	suite.repository.On("GetContinuousScreeningMatch", mock.Anything, mock.Anything,
		suite.matchId).Return(continuousScreeningMatch1, nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil)
	suite.repository.On("GetCaseById", mock.Anything, mock.Anything, suite.caseId.String()).Return(caseData, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningHit", suite.orgId).Return(nil)
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId.String(), false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusConfirmedHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatusByBatch", mock.Anything, mock.Anything,
		[]uuid.UUID{continuousScreeningMatch2.Id, continuousScreeningMatch3.Id},
		models.ScreeningMatchStatusSkipped, mock.Anything).Return(
		[]models.ContinuousScreeningMatch{}, nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusConfirmedHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.CaseId == suite.caseId.String() &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusConfirmedHit.String()
	})).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_ConfirmedHit_WithSingleMatch() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusConfirmedHit,
		ReviewerId: &suite.userId,
	}

	continuousScreeningMatch := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:     suite.screeningId,
			OrgId:  suite.orgId,
			Status: models.ScreeningStatusInReview,
			CaseId: &suite.caseId,
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch},
	}

	caseData := models.Case{
		Id: suite.caseId.String(),
	}

	updatedMatch := continuousScreeningMatch
	updatedMatch.Status = models.ScreeningMatchStatusConfirmedHit

	// Mock expectations
	suite.repository.On("GetContinuousScreeningMatch", mock.Anything, mock.Anything,
		suite.matchId).Return(continuousScreeningMatch, nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil)
	suite.repository.On("GetCaseById", mock.Anything, mock.Anything, suite.caseId.String()).Return(caseData, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningHit", suite.orgId).Return(nil)
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId.String(), false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusConfirmedHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatusByBatch", mock.Anything, mock.Anything,
		[]uuid.UUID{}, models.ScreeningMatchStatusSkipped, mock.Anything).Return(
		[]models.ContinuousScreeningMatch{}, nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusConfirmedHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.CaseId == suite.caseId.String() &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusConfirmedHit.String()
	})).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_NoHit_WithWhitelist_AndOtherMatches() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusNoHit,
		ReviewerId: &suite.userId,
		Whitelist:  true,
	}

	continuousScreeningMatch1 := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id-1",
	}
	continuousScreeningMatch2 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id-2",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:         suite.screeningId,
			OrgId:      suite.orgId,
			Status:     models.ScreeningStatusInReview,
			CaseId:     &suite.caseId,
			ObjectType: "transactions",
			ObjectId:   "test-object-id",
			IsPartial:  false,
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch1, continuousScreeningMatch2},
	}

	caseData := models.Case{
		Id: suite.caseId.String(),
	}

	updatedMatch := continuousScreeningMatch1
	updatedMatch.Status = models.ScreeningMatchStatusNoHit

	// Mock expectations
	suite.repository.On("GetContinuousScreeningMatch", mock.Anything, mock.Anything,
		suite.matchId).Return(continuousScreeningMatch1, nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil)
	suite.repository.On("GetCaseById", mock.Anything, mock.Anything, suite.caseId.String()).Return(caseData, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningHit", suite.orgId).Return(nil)
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId.String(), false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.enforceSecurity.On("WriteWhitelist", mock.Anything).Return(nil)
	suite.repository.On("AddScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId.String(), "transactions_test-object-id", "test-entity-id-1", &suite.userId).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_NoHit_LastMatch_ChangesScreeningStatus() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusNoHit,
		ReviewerId: &suite.userId,
		Whitelist:  false,
	}

	continuousScreeningMatch := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:     suite.screeningId,
			OrgId:  suite.orgId,
			Status: models.ScreeningStatusInReview,
			CaseId: &suite.caseId,
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch},
	}

	caseData := models.Case{
		Id: suite.caseId.String(),
	}

	updatedMatch := continuousScreeningMatch
	updatedMatch.Status = models.ScreeningMatchStatusNoHit

	// Mock expectations
	suite.repository.On("GetContinuousScreeningMatch", mock.Anything, mock.Anything,
		suite.matchId).Return(continuousScreeningMatch, nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil)
	suite.repository.On("GetCaseById", mock.Anything, mock.Anything, suite.caseId.String()).Return(caseData, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningHit", suite.orgId).Return(nil)
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId.String(), false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusNoHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.CaseId == suite.caseId.String() &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusNoHit.String()
	})).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_NoHit_IsPartial_NoStatusChange() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusNoHit,
		ReviewerId: &suite.userId,
		Whitelist:  false,
	}

	continuousScreeningMatch := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:        suite.screeningId,
			OrgId:     suite.orgId,
			Status:    models.ScreeningStatusInReview,
			CaseId:    &suite.caseId,
			IsPartial: true, // This should prevent status change
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch},
	}

	caseData := models.Case{
		Id: suite.caseId.String(),
	}

	updatedMatch := continuousScreeningMatch
	updatedMatch.Status = models.ScreeningMatchStatusNoHit

	// Mock expectations
	suite.repository.On("GetContinuousScreeningMatch", mock.Anything, mock.Anything,
		suite.matchId).Return(continuousScreeningMatch, nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil)
	suite.repository.On("GetCaseById", mock.Anything, mock.Anything, suite.caseId.String()).Return(caseData, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningHit", suite.orgId).Return(nil)
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId.String(), false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	// Note: No UpdateContinuousScreeningStatus call expected because IsPartial is true

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_InvalidStatus() {
	// Setup - invalid status
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusPending, // Invalid status for update
		ReviewerId: &suite.userId,
	}

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "invalid status received for screening match")
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestDismissContinuousScreening_InsufficientPermissions() {
	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:    suite.screeningId,
			OrgId: suite.orgId,
		},
		Matches: []models.ContinuousScreeningMatch{},
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil)
	suite.enforceSecurity.On("DismissContinuousScreeningHits", suite.orgId).Return(models.ForbiddenError)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.DismissContinuousScreening(suite.ctx, suite.screeningId, &suite.userId)

	// Assert
	suite.Error(err)
	suite.Equal(models.ForbiddenError, err)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestDismissContinuousScreening_NoHitChangesOthersToSkipped() {
	// Setup
	match1 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusNoHit,
	}
	match2 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
	}
	match3 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:     suite.screeningId,
			OrgId:  suite.orgId,
			CaseId: &suite.caseId,
			Status: models.ScreeningStatusInReview,
		},
		Matches: []models.ContinuousScreeningMatch{match1, match2, match3},
	}

	// Expected result after dismissal
	expectedMatch2 := match2
	expectedMatch2.Status = models.ScreeningMatchStatusSkipped
	expectedMatch3 := match3
	expectedMatch3.Status = models.ScreeningMatchStatusSkipped

	expectedResult := continuousScreeningWithMatches
	expectedResult.Matches = []models.ContinuousScreeningMatch{match1, expectedMatch2, expectedMatch3}

	// Mock expectations
	suite.enforceSecurity.On("DismissContinuousScreeningHits", suite.orgId).Return(nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil).Once()
	suite.repository.On("UpdateContinuousScreeningMatchStatusByBatch",
		mock.Anything, mock.Anything, []uuid.UUID{match2.Id, match3.Id},
		models.ScreeningMatchStatusSkipped, mock.Anything).Return(
		[]models.ContinuousScreeningMatch{}, nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusNoHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(expectedResult, nil).Once()

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.DismissContinuousScreening(suite.ctx, suite.screeningId, &suite.userId)

	// Assert
	suite.NoError(err)
	suite.Equal(expectedResult, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestDismissContinuousScreening_ConfirmedHit_NoUpdates() {
	// Setup
	match1 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusConfirmedHit,
	}
	match2 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusSkipped,
	}
	match3 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusSkipped,
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:     suite.screeningId,
			OrgId:  suite.orgId,
			CaseId: &suite.caseId,
			Status: models.ScreeningStatusInReview,
		},
		Matches: []models.ContinuousScreeningMatch{match1, match2, match3},
	}

	// Mock expectations - no pending matches, so batch update with empty slice
	suite.enforceSecurity.On("DismissContinuousScreeningHits", suite.orgId).Return(nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil).Once()
	suite.repository.On("UpdateContinuousScreeningMatchStatusByBatch",
		mock.Anything, mock.Anything, []uuid.UUID{}, models.ScreeningMatchStatusSkipped,
		mock.Anything).Return([]models.ContinuousScreeningMatch{}, nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusNoHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil).Once()

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.DismissContinuousScreening(suite.ctx, suite.screeningId, &suite.userId)

	// Assert
	suite.NoError(err)
	suite.Equal(continuousScreeningWithMatches, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestDismissContinuousScreening_NotInCase() {
	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:     suite.screeningId,
			OrgId:  suite.orgId,
			CaseId: nil, // Not in case
			Status: models.ScreeningStatusInReview,
		},
		Matches: []models.ContinuousScreeningMatch{},
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil)
	suite.enforceSecurity.On("DismissContinuousScreeningHits", suite.orgId).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.DismissContinuousScreening(suite.ctx, suite.screeningId, &suite.userId)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "continuous screening is not in case, can't dismiss")
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestDismissContinuousScreening_NotInReview() {
	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:     suite.screeningId,
			OrgId:  suite.orgId,
			CaseId: &suite.caseId,
			Status: models.ScreeningStatusConfirmedHit, // Not in review
		},
		Matches: []models.ContinuousScreeningMatch{},
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil)
	suite.enforceSecurity.On("DismissContinuousScreeningHits", suite.orgId).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.DismissContinuousScreening(suite.ctx, suite.screeningId, &suite.userId)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "continuous screening is not in review, can't dismiss")
	suite.AssertExpectations()
}
