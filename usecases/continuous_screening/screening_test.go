package continuous_screening

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ScreeningTestSuite struct {
	suite.Suite
	enforceSecurity              *mocks.EnforceSecurity
	repository                   *mocks.ContinuousScreeningRepository
	taskQueueRepository          *mocks.TaskQueueRepository
	clientDbRepository           *mocks.ContinuousScreeningClientDbRepository
	organizationSchemaRepository *mocks.OrganizationSchemaRepository
	ingestedDataReader           *mocks.IngestedDataReader
	ingestionUsecase             *mocks.ContinuousScreeningIngestionUsecase
	screeningProvider            *mocks.OpenSanctionsRepository
	caseEditor                   *mocks.CaseEditor
	objectRiskTopic              *mocks.ObjectRiskTopic
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
	suite.taskQueueRepository = new(mocks.TaskQueueRepository)
	suite.clientDbRepository = new(mocks.ContinuousScreeningClientDbRepository)
	suite.organizationSchemaRepository = new(mocks.OrganizationSchemaRepository)
	suite.ingestedDataReader = new(mocks.IngestedDataReader)
	suite.ingestionUsecase = new(mocks.ContinuousScreeningIngestionUsecase)
	suite.screeningProvider = new(mocks.OpenSanctionsRepository)
	suite.caseEditor = new(mocks.CaseEditor)
	suite.objectRiskTopic = new(mocks.ObjectRiskTopic)

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
		taskQueueRepository:          suite.taskQueueRepository,
		clientDbRepository:           suite.clientDbRepository,
		organizationSchemaRepository: suite.organizationSchemaRepository,
		ingestedDataReader:           suite.ingestedDataReader,
		ingestionUsecase:             suite.ingestionUsecase,
		screeningProvider:            suite.screeningProvider,
		caseEditor:                   suite.caseEditor,
		inboxReader:                  suite.repository,
		objectRiskTopicWriter:        suite.objectRiskTopic,
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
	suite.objectRiskTopic.AssertExpectations(t)
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

	objectType := "test-object-type"
	objectId := "test-object-id"
	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:          suite.screeningId,
			OrgId:       suite.orgId,
			Status:      models.ScreeningStatusInReview,
			CaseId:      &suite.caseId,
			IsPartial:   false,
			TriggerType: models.ContinuousScreeningTriggerTypeObjectAdded,
			ObjectType:  &objectType,
			ObjectId:    &objectId,
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusConfirmedHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusConfirmedHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusConfirmedHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.screeningId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusConfirmedHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningStatusInReview.String()
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatusByBatch", mock.Anything, mock.Anything,
		mock.MatchedBy(func(ids []uuid.UUID) bool {
			return len(ids) == 2
		}), models.ScreeningMatchStatusSkipped, mock.Anything).Return(
		[]models.ContinuousScreeningMatch{}, nil)
	suite.repository.On("BatchCreateCaseEvents", mock.Anything, mock.Anything,
		mock.MatchedBy(func(events []models.CreateCaseEventAttributes) bool {
			return len(events) == 2 &&
				events[0].OrgId == suite.orgId &&
				events[0].CaseId == suite.caseId.String() &&
				events[0].UserId != nil && *events[0].UserId == string(suite.userId) &&
				events[0].EventType == models.ScreeningMatchReviewed &&
				events[0].ResourceId != nil && *events[0].ResourceId ==
				continuousScreeningMatch2.Id.String() &&
				events[0].ResourceType != nil && *events[0].ResourceType ==
				models.ContinuousScreeningMatchResourceType &&
				events[0].NewValue != nil && *events[0].NewValue ==
				models.ScreeningMatchStatusSkipped.String() &&
				events[0].PreviousValue != nil && *events[0].PreviousValue ==
				continuousScreeningMatch2.Status.String() &&
				events[1].OrgId == suite.orgId &&
				events[1].CaseId == suite.caseId.String() &&
				events[1].UserId != nil && *events[1].UserId == string(suite.userId) &&
				events[1].EventType == models.ScreeningMatchReviewed &&
				events[1].ResourceId != nil && *events[1].ResourceId ==
				continuousScreeningMatch3.Id.String() &&
				events[1].ResourceType != nil && *events[1].ResourceType ==
				models.ContinuousScreeningMatchResourceType &&
				events[1].NewValue != nil && *events[1].NewValue ==
				models.ScreeningMatchStatusSkipped.String() &&
				events[1].PreviousValue != nil && *events[1].PreviousValue ==
				continuousScreeningMatch3.Status.String()
		})).Return([]models.CaseEvent{}, nil)

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

	objectType := "test-object-type"
	objectId := "test-object-id"
	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:          suite.screeningId,
			OrgId:       suite.orgId,
			Status:      models.ScreeningStatusInReview,
			CaseId:      &suite.caseId,
			TriggerType: models.ContinuousScreeningTriggerTypeObjectAdded,
			ObjectType:  &objectType,
			ObjectId:    &objectId,
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusConfirmedHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusConfirmedHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusConfirmedHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.screeningId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningStatusConfirmedHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningStatusInReview.String()
	})).Return(models.CaseEvent{}, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_ConfirmedHit_WithRiskTopics() {
	// Setup - This test verifies that when a match is confirmed as a hit and contains
	// risk topics in its payload, the topics are extracted and written via objectRiskTopic
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusConfirmedHit,
		ReviewerId: &suite.userId,
	}

	// Payload with OpenSanctions topics that map to Marble risk topics
	matchPayload := []byte(`{"properties": {"topics": ["sanctions", "pep"]}}`)

	objectType := "test-object-type"
	objectId := "test-object-id"
	continuousScreeningMatch := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "test-entity-id",
		Payload:               matchPayload,
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:          suite.screeningId,
			OrgId:       suite.orgId,
			Status:      models.ScreeningStatusInReview,
			CaseId:      &suite.caseId,
			TriggerType: models.ContinuousScreeningTriggerTypeObjectAdded,
			ObjectType:  &objectType,
			ObjectId:    &objectId,
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusConfirmedHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusConfirmedHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.EventType == models.ScreeningMatchReviewed
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.EventType == models.ScreeningReviewed
	})).Return(models.CaseEvent{}, nil)

	// Expect AppendObjectRiskTopics to be called with the extracted topics
	// "sanctions" -> RiskTopicSanctions, "pep" -> RiskTopicPEPs
	suite.objectRiskTopic.On("AppendObjectRiskTopics", mock.Anything, mock.Anything,
		mock.MatchedBy(func(input models.ObjectRiskTopicWithEventUpsert) bool {
			if input.OrgId != suite.orgId ||
				input.ObjectType != objectType ||
				input.ObjectId != objectId ||
				len(input.Topics) != 2 {
				return false
			}
			// Check that both expected topics are present
			hasSanctions := false
			hasPEPs := false
			for _, topic := range input.Topics {
				if topic == models.RiskTopicSanctions {
					hasSanctions = true
				}
				if topic == models.RiskTopicPEPs {
					hasPEPs = true
				}
			}
			return hasSanctions && hasPEPs
		})).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_DatasetUpdated_ConfirmedHit_WithRiskTopics() {
	// Setup - This test verifies that when a dataset-triggered match is confirmed as a hit,
	// the topics are extracted from the SCREENING's entity payload (not the match payload)
	// and the object type/id come from the MATCH's metadata
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusConfirmedHit,
		ReviewerId: &suite.userId,
	}

	// For dataset-triggered: topics come from screening.OpenSanctionEntityPayload
	screeningEntityPayload := []byte(`{"properties": {"topics": ["pep", "regulatory"]}}`)

	// For dataset-triggered: object type/id come from match.Metadata
	objectType := "test-object-type"
	objectId := "test-object-id"
	continuousScreeningMatch := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "marble-entity-123",
		Metadata: &models.EntityNoteMetadata{
			ObjectType: objectType,
			ObjectId:   objectId,
		},
	}

	openSanctionEntityId := "open-sanction-entity-abc"
	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                        suite.screeningId,
			OrgId:                     suite.orgId,
			Status:                    models.ScreeningStatusInReview,
			CaseId:                    &suite.caseId,
			TriggerType:               models.ContinuousScreeningTriggerTypeDatasetUpdated,
			OpenSanctionEntityId:      &openSanctionEntityId,
			OpenSanctionEntityPayload: screeningEntityPayload,
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusConfirmedHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusConfirmedHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.EventType == models.ScreeningMatchReviewed
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.EventType == models.ScreeningReviewed
	})).Return(models.CaseEvent{}, nil)

	// Expect AppendObjectRiskTopics to be called with topics from screening.OpenSanctionEntityPayload
	// "pep" -> RiskTopicPEPs, "regulatory" -> RiskTopicAdverseMedia
	suite.objectRiskTopic.On("AppendObjectRiskTopics", mock.Anything, mock.Anything,
		mock.MatchedBy(func(input models.ObjectRiskTopicWithEventUpsert) bool {
			if input.OrgId != suite.orgId ||
				input.ObjectType != objectType ||
				input.ObjectId != objectId ||
				len(input.Topics) != 2 {
				return false
			}
			// Check that both expected topics are present
			hasPEPs := false
			hasAdverseMedia := false
			for _, topic := range input.Topics {
				if topic == models.RiskTopicPEPs {
					hasPEPs = true
				}
				if topic == models.RiskTopicAdverseMedia {
					hasAdverseMedia = true
				}
			}
			return hasPEPs && hasAdverseMedia
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
			Id:          suite.screeningId,
			OrgId:       suite.orgId,
			Status:      models.ScreeningStatusInReview,
			CaseId:      &suite.caseId,
			ObjectType:  utils.Ptr("transactions"),
			ObjectId:    utils.Ptr("test-object-id"),
			TriggerType: models.ContinuousScreeningTriggerTypeObjectAdded,
			IsPartial:   false,
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.enforceSecurity.On("WriteWhitelist", mock.Anything).Return(nil)
	suite.repository.On("AddScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, "marble_transactions_test-object-id", "test-entity-id-1", &suite.userId).Return(nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)

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
			Id:          suite.screeningId,
			OrgId:       suite.orgId,
			Status:      models.ScreeningStatusInReview,
			CaseId:      &suite.caseId,
			ObjectType:  utils.Ptr("transactions"),
			ObjectId:    utils.Ptr("test-object-id"),
			TriggerType: models.ContinuousScreeningTriggerTypeObjectAdded,
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch},
	}

	// Expect whitelist creation on NoHit
	suite.enforceSecurity.On("WriteWhitelist", mock.Anything).Return(nil)
	suite.repository.On("AddScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, "marble_transactions_test-object-id", "test-entity-id", &suite.userId).Return(nil)

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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusNoHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.screeningId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningResourceType &&
			attrs.NewValue != nil && *attrs.NewValue == models.ScreeningStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningStatusInReview.String()
	})).Return(models.CaseEvent{}, nil)
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_NoHit_DatasetUpdated_WhitelistsEntity() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusNoHit,
		ReviewerId: &suite.userId,
	}

	continuousScreeningMatch := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "marble-entity-123",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                   suite.screeningId,
			OrgId:                suite.orgId,
			Status:               models.ScreeningStatusInReview,
			CaseId:               &suite.caseId,
			TriggerType:          models.ContinuousScreeningTriggerTypeDatasetUpdated,
			OpenSanctionEntityId: utils.Ptr("open-sanction-entity-abc"),
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	// Whitelist expectations: despite Whitelist=false, we still whitelist
	suite.enforceSecurity.On("WriteWhitelist", mock.Anything).Return(nil)
	suite.repository.On("AddScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, "marble-entity-123", "open-sanction-entity-abc", &suite.userId).Return(nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusNoHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.screeningId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningResourceType &&
			attrs.NewValue != nil && *attrs.NewValue == models.ScreeningStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningStatusInReview.String()
	})).Return(models.CaseEvent{}, nil)

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
			Id:          suite.screeningId,
			OrgId:       suite.orgId,
			Status:      models.ScreeningStatusInReview,
			CaseId:      &suite.caseId,
			IsPartial:   true, // This should prevent status change
			ObjectType:  utils.Ptr("transactions"),
			ObjectId:    utils.Ptr("test-object-id"),
			TriggerType: models.ContinuousScreeningTriggerTypeObjectAdded,
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch},
	}

	// Expect whitelist creation on NoHit even if IsPartial
	suite.enforceSecurity.On("WriteWhitelist", mock.Anything).Return(nil)
	suite.repository.On("AddScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, "marble_transactions_test-object-id", "test-entity-id", &suite.userId).Return(nil)

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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)
	// Note: No UpdateContinuousScreeningStatus call expected because IsPartial is true
	// Note: No ScreeningReviewed event expected because IsPartial prevents status change

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
	suite.repository.On("BatchCreateCaseEvents", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs []models.CreateCaseEventAttributes,
	) bool {
		if len(attrs) != 2 {
			return false
		}
		// Check first event for match2
		if attrs[0].OrgId != suite.orgId ||
			attrs[0].CaseId != suite.caseId.String() ||
			attrs[0].EventType != models.ScreeningMatchReviewed ||
			attrs[0].ResourceId == nil || *attrs[0].ResourceId != match2.Id.String() ||
			attrs[0].ResourceType == nil || *attrs[0].ResourceType !=
			models.ContinuousScreeningMatchResourceType ||
			attrs[0].NewValue == nil || *attrs[0].NewValue !=
			models.ScreeningMatchStatusSkipped.String() ||
			attrs[0].PreviousValue == nil || *attrs[0].PreviousValue != match2.Status.String() {
			return false
		}
		// Check second event for match3
		if attrs[1].OrgId != suite.orgId ||
			attrs[1].CaseId != suite.caseId.String() ||
			attrs[1].EventType != models.ScreeningMatchReviewed ||
			attrs[1].ResourceId == nil || *attrs[1].ResourceId != match3.Id.String() ||
			attrs[1].ResourceType == nil || *attrs[1].ResourceType !=
			models.ContinuousScreeningMatchResourceType ||
			attrs[1].NewValue == nil || *attrs[1].NewValue !=
			models.ScreeningMatchStatusSkipped.String() ||
			attrs[1].PreviousValue == nil || *attrs[1].PreviousValue != match3.Status.String() {
			return false
		}
		return true
	})).Return([]models.CaseEvent{}, nil)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusNoHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.screeningId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningResourceType &&
			attrs.NewValue != nil && *attrs.NewValue == models.ScreeningStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningStatusInReview.String()
	})).Return(models.CaseEvent{}, nil)
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

	// Mock expectations - no pending matches, so no batch update calls
	suite.enforceSecurity.On("DismissContinuousScreeningHits", suite.orgId).Return(nil)
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything,
		suite.screeningId).Return(continuousScreeningWithMatches, nil).Once()
	// Note: UpdateContinuousScreeningMatchStatusByBatch and BatchCreateCaseEvents should NOT be called
	// because there are no pending matches to update
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusNoHit).Return(models.ContinuousScreening{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.screeningId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningResourceType &&
			attrs.NewValue != nil && *attrs.NewValue == models.ScreeningStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningStatusInReview.String()
	})).Return(models.CaseEvent{}, nil)
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

func (suite *ScreeningTestSuite) TestLoadMoreContinuousScreeningMatches() {
	// Setup
	configId := uuid.New()
	stableId := uuid.New()

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName

	table := models.Table{
		Name:      "person",
		FTMEntity: &ftmEntityValue,
		Fields: map[string]models.Field{
			"id":   {Name: "id"},
			"name": {Name: "name", FTMProperty: &ftmPropertyValue},
		},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"person": table,
		},
	}

	ingestedObject := models.DataModelObject{
		Data: map[string]any{
			"name": "test person",
		},
		Metadata: map[string]any{
			"id": [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		},
	}

	screening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                                suite.screeningId,
			OrgId:                             suite.orgId,
			ContinuousScreeningConfigId:       configId,
			ContinuousScreeningConfigStableId: stableId,
			Status:                            models.ScreeningStatusInReview,
			IsPartial:                         true,
			NumberOfMatches:                   3,
			SearchInput:                       []byte(`{"name": ["test"]}`),
			ObjectType:                        utils.Ptr("person"),
			ObjectId:                          utils.Ptr("test-object-id"),
			TriggerType:                       models.ContinuousScreeningTriggerTypeObjectAdded,
		},
		Matches: []models.ContinuousScreeningMatch{
			{OpenSanctionEntityId: "existing_1"},
			{OpenSanctionEntityId: "existing_2"},
			{OpenSanctionEntityId: "existing_3"},
		},
	}

	config := models.ContinuousScreeningConfig{
		Id:             configId,
		StableId:       stableId,
		OrgId:          suite.orgId,
		Name:           "Test Config",
		Datasets:       []string{"dataset1"},
		MatchThreshold: 80,
		MatchLimit:     10,
	}

	newMatches := []models.ScreeningMatch{
		{EntityId: "existing_1"}, // Should be deduplicated
		{EntityId: "existing_2"}, // Should be deduplicated
		{EntityId: "existing_3"}, // Should be deduplicated
		{EntityId: "new_1"},
		{EntityId: "new_2"},
		{EntityId: "new_3"},
		{EntityId: "new_4"},
	}

	searchResponse := models.ScreeningRawSearchResponseWithMatches{
		Matches: newMatches,
		Count:   7,
		Partial: false,
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything, suite.screeningId).
		Return(screening, nil)

	suite.enforceSecurity.On("WriteContinuousScreeningHit", suite.orgId).
		Return(nil)

	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything, stableId).
		Return(config, nil)

	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId, false, false).
		Return(dataModel, nil)

	suite.repository.On("SearchScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, mock.Anything, mock.Anything).
		Return([]models.ScreeningWhitelist{}, nil)

	suite.ingestedDataReader.On("QueryIngestedObject", mock.Anything, mock.Anything, table, "test-object-id", mock.Anything).
		Return([]models.DataModelObject{ingestedObject}, nil)

	suite.screeningProvider.On("Search", mock.Anything, mock.MatchedBy(func(q models.OpenSanctionsQuery) bool {
		return q.OrgConfig.MatchLimit == 500 && q.Config.Datasets[0] == "dataset1"
	})).Return(searchResponse, nil)

	insertedMatches := []models.ContinuousScreeningMatch{
		{OpenSanctionEntityId: "new_1"},
		{OpenSanctionEntityId: "new_2"},
		{OpenSanctionEntityId: "new_3"},
		{OpenSanctionEntityId: "new_4"},
	}

	suite.repository.On("InsertContinuousScreeningMatches", mock.Anything, mock.Anything,
		suite.screeningId, mock.MatchedBy(func(matches []models.ContinuousScreeningMatch) bool {
			return len(matches) == 4 &&
				matches[0].OpenSanctionEntityId == "new_1" &&
				matches[1].OpenSanctionEntityId == "new_2" &&
				matches[2].OpenSanctionEntityId == "new_3" &&
				matches[3].OpenSanctionEntityId == "new_4"
		})).Return(insertedMatches, nil)

	suite.repository.On("UpdateContinuousScreening", mock.Anything, mock.Anything,
		suite.screeningId, mock.MatchedBy(func(input models.UpdateContinuousScreeningInput) bool {
			return input.IsPartial != nil && *input.IsPartial == false &&
				input.NumberOfMatches != nil && *input.NumberOfMatches == 7
		})).Return(models.ContinuousScreening{}, nil)

	suite.taskQueueRepository.On("EnqueueContinuousScreeningMatchEnrichmentTask",
		mock.Anything, mock.Anything, suite.orgId, suite.screeningId).
		Return(nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.LoadMoreContinuousScreeningMatches(suite.ctx, suite.screeningId)

	// Assert
	suite.NoError(err)
	suite.Equal(7, result.NumberOfMatches)
	suite.False(result.IsPartial)
	suite.Len(result.Matches, 7) // 3 existing + 4 new
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestLoadMoreContinuousScreeningMatches_NotInReview() {
	// Setup - screening status is not InReview
	screening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:        suite.screeningId,
			OrgId:     suite.orgId,
			Status:    models.ScreeningStatusConfirmedHit, // Not InReview
			IsPartial: true,
		},
		Matches: []models.ContinuousScreeningMatch{},
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything, suite.screeningId).
		Return(screening, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningHit", suite.orgId).
		Return(nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.LoadMoreContinuousScreeningMatches(suite.ctx, suite.screeningId)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "continuous screening is not in review, can't load more results")
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestLoadMoreContinuousScreeningMatches_NotPartial() {
	// Setup - screening is not partial
	screening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:        suite.screeningId,
			OrgId:     suite.orgId,
			Status:    models.ScreeningStatusInReview,
			IsPartial: false, // Not partial
		},
		Matches: []models.ContinuousScreeningMatch{},
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything, suite.screeningId).
		Return(screening, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningHit", suite.orgId).
		Return(nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.LoadMoreContinuousScreeningMatches(suite.ctx, suite.screeningId)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "continuous screening is not partial, can't load more results")
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestLoadMoreContinuousScreeningMatches_NoNewMatches() {
	// Setup
	configId := uuid.New()
	stableId := uuid.New()

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName

	table := models.Table{
		Name:      "person",
		FTMEntity: &ftmEntityValue,
		Fields: map[string]models.Field{
			"id":   {Name: "id"},
			"name": {Name: "name", FTMProperty: &ftmPropertyValue},
		},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"person": table,
		},
	}

	ingestedObject := models.DataModelObject{
		Data: map[string]any{
			"name": "test person",
		},
		Metadata: map[string]any{
			"id": [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		},
	}

	screening := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                                suite.screeningId,
			OrgId:                             suite.orgId,
			ContinuousScreeningConfigId:       configId,
			ContinuousScreeningConfigStableId: stableId,
			Status:                            models.ScreeningStatusInReview,
			IsPartial:                         true,
			NumberOfMatches:                   3,
			SearchInput:                       []byte(`{"name": ["test"]}`),
			ObjectType:                        utils.Ptr("person"),
			ObjectId:                          utils.Ptr("test-object-id"),
			TriggerType:                       models.ContinuousScreeningTriggerTypeObjectAdded,
		},
		Matches: []models.ContinuousScreeningMatch{
			{OpenSanctionEntityId: "existing_1"},
			{OpenSanctionEntityId: "existing_2"},
			{OpenSanctionEntityId: "existing_3"},
		},
	}

	config := models.ContinuousScreeningConfig{
		Id:             configId,
		StableId:       stableId,
		OrgId:          suite.orgId,
		Name:           "Test Config",
		Datasets:       []string{"dataset1"},
		MatchThreshold: 80,
		MatchLimit:     10,
	}

	// All matches are duplicates of existing matches
	duplicateMatches := []models.ScreeningMatch{
		{EntityId: "existing_1"},
		{EntityId: "existing_2"},
		{EntityId: "existing_3"},
	}

	searchResponse := models.ScreeningRawSearchResponseWithMatches{
		Matches: duplicateMatches,
		Count:   3,
		Partial: false,
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningWithMatchesById", mock.Anything, mock.Anything, suite.screeningId).
		Return(screening, nil)

	suite.enforceSecurity.On("WriteContinuousScreeningHit", suite.orgId).
		Return(nil)

	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything, stableId).
		Return(config, nil)

	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId, false, false).
		Return(dataModel, nil)

	suite.repository.On("SearchScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, mock.Anything, mock.Anything).
		Return([]models.ScreeningWhitelist{}, nil)

	suite.ingestedDataReader.On("QueryIngestedObject", mock.Anything, mock.Anything, table, "test-object-id", mock.Anything).
		Return([]models.DataModelObject{ingestedObject}, nil)

	suite.screeningProvider.On("Search", mock.Anything, mock.MatchedBy(func(q models.OpenSanctionsQuery) bool {
		return q.OrgConfig.MatchLimit == 500 && q.Config.Datasets[0] == "dataset1"
	})).Return(searchResponse, nil)

	// InsertContinuousScreeningMatches called with empty slice (no new matches)
	suite.repository.On("InsertContinuousScreeningMatches", mock.Anything, mock.Anything,
		suite.screeningId, []models.ContinuousScreeningMatch{}).Return(
		[]models.ContinuousScreeningMatch{}, nil)

	suite.repository.On("UpdateContinuousScreening", mock.Anything, mock.Anything,
		suite.screeningId, mock.MatchedBy(func(input models.UpdateContinuousScreeningInput) bool {
			return input.IsPartial != nil && *input.IsPartial == false &&
				input.NumberOfMatches != nil && *input.NumberOfMatches == 3 // No change in count
		})).Return(models.ContinuousScreening{}, nil)

	suite.taskQueueRepository.On("EnqueueContinuousScreeningMatchEnrichmentTask",
		mock.Anything, mock.Anything, suite.orgId, suite.screeningId).
		Return(nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.LoadMoreContinuousScreeningMatches(suite.ctx, suite.screeningId)

	// Assert
	suite.NoError(err)
	suite.Equal(3, result.NumberOfMatches) // No new matches added
	suite.False(result.IsPartial)
	suite.Len(result.Matches, 3) // Still only existing matches
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_DatasetUpdated_ConfirmedHit_NotLastMatch() {
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
		OpenSanctionEntityId:  "marble-entity-123",
		Metadata: &models.EntityNoteMetadata{
			ObjectType: "test-object-type",
			ObjectId:   "test-object-id",
		},
	}
	continuousScreeningMatch2 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "marble-entity-456",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                   suite.screeningId,
			OrgId:                suite.orgId,
			Status:               models.ScreeningStatusInReview,
			CaseId:               &suite.caseId,
			IsPartial:            false,
			TriggerType:          models.ContinuousScreeningTriggerTypeDatasetUpdated,
			OpenSanctionEntityId: utils.Ptr("open-sanction-entity-abc"),
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch1, continuousScreeningMatch2},
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusConfirmedHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	// No screening status update since there are more pending matches (not last)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusConfirmedHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_DatasetUpdated_ConfirmedHit_LastMatch() {
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
		OpenSanctionEntityId:  "marble-entity-123",
		Metadata: &models.EntityNoteMetadata{
			ObjectType: "test-object-type",
			ObjectId:   "test-object-id",
		},
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                   suite.screeningId,
			OrgId:                suite.orgId,
			Status:               models.ScreeningStatusInReview,
			CaseId:               &suite.caseId,
			IsPartial:            false,
			TriggerType:          models.ContinuousScreeningTriggerTypeDatasetUpdated,
			OpenSanctionEntityId: utils.Ptr("open-sanction-entity-abc"),
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch1},
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusConfirmedHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	// Update screening status to confirmed_hit (since it's the last match)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusConfirmedHit).Return(models.ContinuousScreening{}, nil)
	// Events for match update and screening status update
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusConfirmedHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.screeningId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningStatusConfirmedHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningStatusInReview.String()
	})).Return(models.CaseEvent{}, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_DatasetUpdated_NoHit_NotLastMatch() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusNoHit,
		ReviewerId: &suite.userId,
	}

	continuousScreeningMatch1 := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "marble-entity-123",
	}
	continuousScreeningMatch2 := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "marble-entity-456",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                   suite.screeningId,
			OrgId:                suite.orgId,
			Status:               models.ScreeningStatusInReview,
			CaseId:               &suite.caseId,
			IsPartial:            false,
			TriggerType:          models.ContinuousScreeningTriggerTypeDatasetUpdated,
			OpenSanctionEntityId: utils.Ptr("open-sanction-entity-abc"),
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	suite.enforceSecurity.On("WriteWhitelist", mock.Anything).Return(nil)
	suite.repository.On("AddScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, "marble-entity-123", "open-sanction-entity-abc", &suite.userId).Return(nil)
	// No screening status update since there are more pending matches (not last)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_DatasetUpdated_NoHit_LastMatch_NoConfirmedHit() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusNoHit,
		ReviewerId: &suite.userId,
	}

	continuousScreeningMatch1 := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "marble-entity-123",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                   suite.screeningId,
			OrgId:                suite.orgId,
			Status:               models.ScreeningStatusInReview,
			CaseId:               &suite.caseId,
			IsPartial:            false,
			TriggerType:          models.ContinuousScreeningTriggerTypeDatasetUpdated,
			OpenSanctionEntityId: utils.Ptr("open-sanction-entity-abc"),
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch1},
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	// Update screening status to no_hit (since it's the last match and no confirmed hits)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusNoHit).Return(models.ContinuousScreening{}, nil)
	// Whitelist creation
	suite.enforceSecurity.On("WriteWhitelist", mock.Anything).Return(nil)
	suite.repository.On("AddScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, "marble-entity-123", "open-sanction-entity-abc", &suite.userId).Return(nil)
	// Events for match update and screening status update
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.screeningId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningResourceType &&
			attrs.NewValue != nil && *attrs.NewValue == models.ScreeningStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningStatusInReview.String()
	})).Return(models.CaseEvent{}, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_DatasetUpdated_NoHit_LastMatch_WithConfirmedHit() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusNoHit,
		ReviewerId: &suite.userId,
	}

	confirmedMatch := models.ContinuousScreeningMatch{
		Id:                    uuid.New(),
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusConfirmedHit,
		OpenSanctionEntityId:  "marble-entity-111",
	}

	continuousScreeningMatch1 := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "marble-entity-123",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                   suite.screeningId,
			OrgId:                suite.orgId,
			Status:               models.ScreeningStatusInReview,
			CaseId:               &suite.caseId,
			IsPartial:            false,
			TriggerType:          models.ContinuousScreeningTriggerTypeDatasetUpdated,
			OpenSanctionEntityId: utils.Ptr("open-sanction-entity-abc"),
		},
		Matches: []models.ContinuousScreeningMatch{confirmedMatch, continuousScreeningMatch1},
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	// Update screening status to confirmed_hit (since there's a confirmed hit and it's the last match)
	suite.repository.On("UpdateContinuousScreeningStatus", mock.Anything, mock.Anything,
		suite.screeningId, models.ScreeningStatusConfirmedHit).Return(models.ContinuousScreening{}, nil)
	// Whitelist creation
	suite.enforceSecurity.On("WriteWhitelist", mock.Anything).Return(nil)
	suite.repository.On("AddScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, "marble-entity-123", "open-sanction-entity-abc", &suite.userId).Return(nil)
	// Events for match update and screening status update
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusNoHit.String()
	})).Return(models.CaseEvent{}, nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.EventType == models.ScreeningReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.screeningId.String() &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningStatusConfirmedHit.String()
	})).Return(models.CaseEvent{}, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_DatasetUpdated_ConfirmedHit_IsPartial() {
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
		OpenSanctionEntityId:  "marble-entity-123",
		Metadata: &models.EntityNoteMetadata{
			ObjectType: "test-object-type",
			ObjectId:   "test-object-id",
		},
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                   suite.screeningId,
			OrgId:                suite.orgId,
			Status:               models.ScreeningStatusInReview,
			CaseId:               &suite.caseId,
			IsPartial:            true, // Partial results
			TriggerType:          models.ContinuousScreeningTriggerTypeDatasetUpdated,
			OpenSanctionEntityId: utils.Ptr("open-sanction-entity-abc"),
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch1},
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusConfirmedHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	// No screening status update since isPartial=True (more results could be loaded)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusConfirmedHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}

func (suite *ScreeningTestSuite) TestUpdateContinuousScreeningMatchStatus_DatasetUpdated_NoHit_IsPartial() {
	// Setup
	input := models.ScreeningMatchUpdate{
		MatchId:    suite.matchId.String(),
		Status:     models.ScreeningMatchStatusNoHit,
		ReviewerId: &suite.userId,
	}

	continuousScreeningMatch1 := models.ContinuousScreeningMatch{
		Id:                    suite.matchId,
		ContinuousScreeningId: suite.screeningId,
		Status:                models.ScreeningMatchStatusPending,
		OpenSanctionEntityId:  "marble-entity-123",
	}

	continuousScreeningWithMatches := models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                   suite.screeningId,
			OrgId:                suite.orgId,
			Status:               models.ScreeningStatusInReview,
			CaseId:               &suite.caseId,
			IsPartial:            true, // Partial results
			TriggerType:          models.ContinuousScreeningTriggerTypeDatasetUpdated,
			OpenSanctionEntityId: utils.Ptr("open-sanction-entity-abc"),
		},
		Matches: []models.ContinuousScreeningMatch{continuousScreeningMatch1},
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
	suite.repository.On("ListInboxes", mock.Anything, mock.Anything, suite.orgId, false).Return([]models.Inbox{}, nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", mock.Anything, mock.Anything).Return(nil)
	suite.repository.On("UpdateContinuousScreeningMatchStatus", mock.Anything, mock.Anything,
		suite.matchId, models.ScreeningMatchStatusNoHit, mock.Anything).Return(updatedMatch, nil)
	suite.caseEditor.On("PerformCaseActionSideEffects", mock.Anything, mock.Anything, caseData).Return(nil)
	// No screening status update since isPartial=True (more results could be loaded)
	// Whitelist creation for no_hit
	suite.enforceSecurity.On("WriteWhitelist", mock.Anything).Return(nil)
	suite.repository.On("AddScreeningMatchWhitelist", mock.Anything, mock.Anything,
		suite.orgId, "marble-entity-123", "open-sanction-entity-abc", &suite.userId).Return(nil)
	suite.repository.On("CreateCaseEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(
		attrs models.CreateCaseEventAttributes,
	) bool {
		return attrs.OrgId == suite.orgId &&
			attrs.CaseId == suite.caseId.String() &&
			attrs.UserId != nil && *attrs.UserId == string(suite.userId) &&
			attrs.EventType == models.ScreeningMatchReviewed &&
			attrs.ResourceId != nil && *attrs.ResourceId == suite.matchId.String() &&
			attrs.ResourceType != nil && *attrs.ResourceType ==
			models.ContinuousScreeningMatchResourceType &&
			attrs.NewValue != nil && *attrs.NewValue ==
			models.ScreeningMatchStatusNoHit.String() &&
			attrs.PreviousValue != nil && *attrs.PreviousValue ==
			models.ScreeningMatchStatusPending.String()
	})).Return(models.CaseEvent{}, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningMatchStatus(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedMatch, result)
	suite.AssertExpectations()
}
