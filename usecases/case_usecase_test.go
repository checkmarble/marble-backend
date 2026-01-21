package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
)

type CaseUsecaseTestSuite struct {
	suite.Suite
	repository           *mocks.CaseRepository
	executorFactory      *mocks.ExecutorFactory
	executor             *mocks.Executor
	enforceSecurity      *mocks.EnforceSecurity
	inboxRepository      *mocks.InboxRepository
	inboxEnforceSecurity *mocks.EnforceSecurity

	organizationId uuid.UUID
}

func (suite *CaseUsecaseTestSuite) SetupTest() {
	suite.repository = new(mocks.CaseRepository)
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.executor = new(mocks.Executor)
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.inboxRepository = new(mocks.InboxRepository)
	suite.inboxEnforceSecurity = new(mocks.EnforceSecurity)

	suite.organizationId = uuid.MustParse("00000000-0000-0000-0000-000000000001")
}

func (suite *CaseUsecaseTestSuite) makeUsecase() *CaseUseCase {
	return &CaseUseCase{
		repository:      suite.repository,
		executorFactory: suite.executorFactory,
		enforceSecurity: suite.enforceSecurity,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity: suite.inboxEnforceSecurity,
			InboxRepository: suite.inboxRepository,
			Credentials: models.Credentials{
				OrganizationId: suite.organizationId,
				Role:           models.ADMIN,
			},
			ExecutorFactory: suite.executorFactory,
		},
	}
}

func (suite *CaseUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.repository.AssertExpectations(t)
	suite.executorFactory.AssertExpectations(t)
	suite.executor.AssertExpectations(t)
	suite.enforceSecurity.AssertExpectations(t)
	suite.inboxRepository.AssertExpectations(t)
	suite.inboxEnforceSecurity.AssertExpectations(t)
}

func (suite *CaseUsecaseTestSuite) Test_GetRelatedContinuousScreeningCasesByOpenSanctionEntityId_WithReferents() {
	ctx := context.Background()

	// Entity IDs
	currentEntityId := "entity-123"
	oldEntityId1 := "entity-100"
	oldEntityId2 := "entity-101"

	// Case IDs
	case1Id := uuid.MustParse("00000000-0000-0000-0000-000000000011")
	case2Id := uuid.MustParse("00000000-0000-0000-0000-000000000012")
	case3Id := uuid.MustParse("00000000-0000-0000-0000-000000000013")

	// Inbox ID
	inboxId := uuid.MustParse("00000000-0000-0000-0000-000000000099")

	// Create entity payload with referents
	entityPayload := models.OpenSanctionsDeltaFileEntity{
		Id:        currentEntityId,
		Caption:   "John Doe",
		Schema:    "Person",
		Referents: []string{oldEntityId1, oldEntityId2},
		Datasets:  []string{"ofac"},
	}
	entityPayloadBytes, _ := json.Marshal(entityPayload)

	// Latest continuous screening with entity payload
	latestScreening := &models.ContinuousScreening{
		Id:                        uuid.MustParse("00000000-0000-0000-0000-000000000020"),
		OrgId:                     suite.organizationId,
		OpenSanctionEntityId:      &currentEntityId,
		OpenSanctionEntityPayload: entityPayloadBytes,
	}

	// Three cases linked to different entity IDs
	case1 := models.Case{
		Id:             case1Id.String(),
		OrganizationId: suite.organizationId,
		InboxId:        inboxId,
		Name:           "Case 1 - Current Entity",
	}
	case2 := models.Case{
		Id:             case2Id.String(),
		OrganizationId: suite.organizationId,
		InboxId:        inboxId,
		Name:           "Case 2 - Old Entity 1",
	}
	case3 := models.Case{
		Id:             case3Id.String(),
		OrganizationId: suite.organizationId,
		InboxId:        inboxId,
		Name:           "Case 3 - Old Entity 2",
	}

	// Mock executor factory
	suite.executorFactory.On("NewExecutor").Return(suite.executor)

	// Mock inbox repository to return available inboxes
	// ListInboxes signature: (ctx, exec, orgId, inboxIds []uuid.UUID, withCaseCount bool)
	suite.inboxRepository.On("ListInboxes", ctx, suite.executor, suite.organizationId,
		[]uuid.UUID(nil), false).
		Return([]models.Inbox{{Id: inboxId, OrganizationId: suite.organizationId}}, nil)

	// Mock inbox security check
	suite.inboxEnforceSecurity.On("ReadInbox", mock.AnythingOfType("models.Inbox")).Return(nil)

	// Mock GetLatestContinuousScreeningByOpenSanctionEntityId to return screening with referents
	suite.repository.On(
		"GetLatestContinuousScreeningByOpenSanctionEntityId",
		ctx,
		suite.executor,
		suite.organizationId,
		currentEntityId,
	).Return(latestScreening, nil)

	// Mock GetContinuousScreeningCasesWithOpenSanctionEntityIds
	// Should be called with current ID + all referent IDs
	expectedEntityIds := []string{currentEntityId, oldEntityId1, oldEntityId2}
	suite.repository.On(
		"GetContinuousScreeningCasesWithOpenSanctionEntityIds",
		ctx,
		suite.executor,
		suite.organizationId,
		expectedEntityIds,
	).Return([]models.Case{case1, case2, case3}, nil)

	// Mock security checks for all cases
	suite.enforceSecurity.On("ReadOrUpdateCase", case1.GetMetadata(), []uuid.UUID{inboxId}).Return(nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", case2.GetMetadata(), []uuid.UUID{inboxId}).Return(nil)
	suite.enforceSecurity.On("ReadOrUpdateCase", case3.GetMetadata(), []uuid.UUID{inboxId}).Return(nil)

	// Execute the usecase method
	cases, err := suite.makeUsecase().GetRelatedContinuousScreeningCasesByOpenSanctionEntityId(
		ctx,
		suite.organizationId,
		currentEntityId,
	)

	// Assertions
	suite.NoError(err)
	suite.Len(cases, 3, "Should return all 3 cases (current + 2 referents)")
	suite.Contains(cases, case1)
	suite.Contains(cases, case2)
	suite.Contains(cases, case3)

	suite.AssertExpectations()
}

func (suite *CaseUsecaseTestSuite) Test_GetRelatedContinuousScreeningCasesByOpenSanctionEntityId_NoReferents() {
	ctx := context.Background()

	currentEntityId := "entity-456"
	case1Id := uuid.MustParse("00000000-0000-0000-0000-000000000021")
	inboxId := uuid.MustParse("00000000-0000-0000-0000-000000000099")

	// Entity payload with empty referents
	entityPayload := models.OpenSanctionsDeltaFileEntity{
		Id:        currentEntityId,
		Caption:   "Jane Smith",
		Schema:    "Person",
		Referents: []string{}, // Empty referents
		Datasets:  []string{"ofac"},
	}
	entityPayloadBytes, _ := json.Marshal(entityPayload)

	latestScreening := &models.ContinuousScreening{
		Id:                        uuid.MustParse("00000000-0000-0000-0000-000000000030"),
		OrgId:                     suite.organizationId,
		OpenSanctionEntityId:      &currentEntityId,
		OpenSanctionEntityPayload: entityPayloadBytes,
	}

	case1 := models.Case{
		Id:             case1Id.String(),
		OrganizationId: suite.organizationId,
		InboxId:        inboxId,
		Name:           "Case 1",
	}

	suite.executorFactory.On("NewExecutor").Return(suite.executor)
	suite.inboxRepository.On("ListInboxes", ctx, suite.executor, suite.organizationId,
		[]uuid.UUID(nil), false).
		Return([]models.Inbox{{Id: inboxId, OrganizationId: suite.organizationId}}, nil)
	suite.inboxEnforceSecurity.On("ReadInbox", mock.AnythingOfType("models.Inbox")).Return(nil)

	suite.repository.On(
		"GetLatestContinuousScreeningByOpenSanctionEntityId",
		ctx,
		suite.executor,
		suite.organizationId,
		currentEntityId,
	).Return(latestScreening, nil)

	// Should only search for the current entity ID
	suite.repository.On(
		"GetContinuousScreeningCasesWithOpenSanctionEntityIds",
		ctx,
		suite.executor,
		suite.organizationId,
		[]string{currentEntityId},
	).Return([]models.Case{case1}, nil)

	suite.enforceSecurity.On("ReadOrUpdateCase", case1.GetMetadata(), []uuid.UUID{inboxId}).Return(nil)

	cases, err := suite.makeUsecase().GetRelatedContinuousScreeningCasesByOpenSanctionEntityId(
		ctx,
		suite.organizationId,
		currentEntityId,
	)

	suite.NoError(err)
	suite.Len(cases, 1, "Should return only 1 case when no referents")
	suite.Equal(case1, cases[0])

	suite.AssertExpectations()
}

func (suite *CaseUsecaseTestSuite) Test_GetRelatedContinuousScreeningCasesByOpenSanctionEntityId_NoLatestScreening() {
	ctx := context.Background()

	currentEntityId := "entity-789"
	case1Id := uuid.MustParse("00000000-0000-0000-0000-000000000031")
	inboxId := uuid.MustParse("00000000-0000-0000-0000-000000000099")

	case1 := models.Case{
		Id:             case1Id.String(),
		OrganizationId: suite.organizationId,
		InboxId:        inboxId,
		Name:           "Case 1",
	}

	suite.executorFactory.On("NewExecutor").Return(suite.executor)
	suite.inboxRepository.On("ListInboxes", ctx, suite.executor, suite.organizationId,
		[]uuid.UUID(nil), false).
		Return([]models.Inbox{{Id: inboxId, OrganizationId: suite.organizationId}}, nil)
	suite.inboxEnforceSecurity.On("ReadInbox", mock.AnythingOfType("models.Inbox")).Return(nil)

	// No latest screening found (returns nil)
	suite.repository.On(
		"GetLatestContinuousScreeningByOpenSanctionEntityId",
		ctx,
		suite.executor,
		suite.organizationId,
		currentEntityId,
	).Return((*models.ContinuousScreening)(nil), nil)

	// Should still search for the provided entity ID
	suite.repository.On(
		"GetContinuousScreeningCasesWithOpenSanctionEntityIds",
		ctx,
		suite.executor,
		suite.organizationId,
		[]string{currentEntityId},
	).Return([]models.Case{case1}, nil)

	suite.enforceSecurity.On("ReadOrUpdateCase", case1.GetMetadata(), []uuid.UUID{inboxId}).Return(nil)

	cases, err := suite.makeUsecase().GetRelatedContinuousScreeningCasesByOpenSanctionEntityId(
		ctx,
		suite.organizationId,
		currentEntityId,
	)

	suite.NoError(err)
	suite.Len(cases, 1, "Should return case even when no latest screening")
	suite.Equal(case1, cases[0])

	suite.AssertExpectations()
}

func TestCaseUsecaseSuite(t *testing.T) {
	suite.Run(t, new(CaseUsecaseTestSuite))
}
