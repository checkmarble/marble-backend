package scoring

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/dto"
	dtoScoring "github.com/checkmarble/marble-backend/dto/scoring"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestValidateAst_ScoreComputation(t *testing.T) {
	uc := ScoringRulesetsUsecase{}
	tree := dto.NodeDto{Name: "ScoreComputation"}
	assert.NoError(t, uc.validateScoringRuleAst(tree))
}

func TestValidateAst_InvalidRoot(t *testing.T) {
	uc := ScoringRulesetsUsecase{}
	tree := dto.NodeDto{Name: "And"}
	err := uc.validateScoringRuleAst(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be `ScoreComputation` or `Switch`")
}

func TestValidateAst_Switch_Empty(t *testing.T) {
	uc := ScoringRulesetsUsecase{}
	tree := dto.NodeDto{
		Name:     "Switch",
		Children: []dto.NodeDto{},
	}
	err := uc.validateScoringRuleAst(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must contain at least one child")
}

func TestValidateAst_Switch_WithScoreComputationChildren(t *testing.T) {
	uc := ScoringRulesetsUsecase{}

	tree := dto.NodeDto{
		Name: "Switch",
		Children: []dto.NodeDto{
			{Name: "ScoreComputation"},
			{Name: "ScoreComputation"},
		},
	}

	err := uc.validateScoringRuleAst(tree)

	require.NoError(t, err)
}

func TestValidateAst_Switch_WrongChildType(t *testing.T) {
	uc := ScoringRulesetsUsecase{}

	tree := dto.NodeDto{
		Name: "Switch",
		Children: []dto.NodeDto{
			{Name: "And"},
		},
	}

	err := uc.validateScoringRuleAst(tree)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all `Switch` children must be `ScoreComputation`")
}

type ScoringRulesetsUsecaseTestSuite struct {
	suite.Suite

	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
	executorFactory    *mocks.ExecutorFactory
	repository         *mocks.ScoringRepository
	taskQueue          *mocks.TaskQueueRepository
	indexEditor        *mocks.ClientDbIndexEditor
	enforceSecurity    *mocks.EnforceSecurity

	orgId      uuid.UUID
	entityType string
	ctx        context.Context
}

func (s *ScoringRulesetsUsecaseTestSuite) SetupTest() {
	s.transaction = new(mocks.Transaction)
	s.transactionFactory = &mocks.TransactionFactory{TxMock: s.transaction}
	s.executorFactory = new(mocks.ExecutorFactory)
	s.repository = new(mocks.ScoringRepository)
	s.taskQueue = new(mocks.TaskQueueRepository)
	s.indexEditor = new(mocks.ClientDbIndexEditor)
	s.enforceSecurity = new(mocks.EnforceSecurity)

	s.orgId = uuid.New()
	s.entityType = "account"
	s.ctx = context.Background()
}

func (s *ScoringRulesetsUsecaseTestSuite) makeUsecase() ScoringRulesetsUsecase {
	return ScoringRulesetsUsecase{
		enforceSecurity:     s.enforceSecurity,
		executorFactory:     s.executorFactory,
		transactionFactory:  s.transactionFactory,
		repository:          s.repository,
		indexEditor:         s.indexEditor,
		taskQueueRepository: s.taskQueue,
	}
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCreateRulesetVersion_InsertsRulesetAndRules() {
	stableId := uuid.New()
	req := dtoScoring.CreateRulesetRequest{
		Name:        "my ruleset",
		Description: "desc",
		Thresholds:  []int{10, 20},
		Rules: []dtoScoring.CreateRuleRequest{
			{
				StableId:    stableId,
				Name:        "rule 1",
				Description: "rule desc",
				Ast:         dto.NodeDto{Name: "ScoreComputation"},
			},
		},
	}

	insertedRuleset := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Version: 1}
	insertedRule := models.ScoringRule{Id: uuid.New(), StableId: stableId, Name: "rule 1"}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetCommitted).
		Return(models.ScoringRuleset{}, models.NotFoundError).Once()
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("InsertScoringRulesetVersion", s.ctx, s.transaction, s.orgId, mock.MatchedBy(func(r models.CreateScoringRulesetRequest) bool {
		return r.Version == 1 && r.Name == req.Name && r.EntityType == s.entityType
	})).Return(insertedRuleset, nil).Once()
	s.repository.On("DeleteAllRulesetRules", s.ctx, s.transaction, mock.MatchedBy(func(r models.ScoringRuleset) bool {
		return r.Version == 1 && r.EntityType == s.entityType
	})).Return(nil).Once()
	s.repository.On("InsertScoringRulesetVersionRule", s.ctx, s.transaction,
		mock.MatchedBy(func(rs models.ScoringRuleset) bool { return rs.Id == insertedRuleset.Id }),
		mock.MatchedBy(func(r models.CreateScoringRuleRequest) bool {
			return r.StableId == stableId && r.Name == "rule 1"
		})).Return(insertedRule, nil).Once()

	result, err := s.makeUsecase().CreateRulesetVersion(s.ctx, s.entityType, req)

	s.NoError(err)
	s.Equal(insertedRuleset.Id, result.Id)
	s.Equal(1, result.Version)

	updatedRuleset := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Version: 2}
	updatedRule := models.ScoringRule{Id: uuid.New(), StableId: stableId, Name: "rule 1"}

	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetCommitted).
		Return(models.ScoringRuleset{Id: insertedRuleset.Id, Version: 1}, nil).Once()
	s.repository.On("InsertScoringRulesetVersion", s.ctx, s.transaction, s.orgId, mock.MatchedBy(func(r models.CreateScoringRulesetRequest) bool {
		return r.Version == 2 && r.Name == req.Name && r.EntityType == s.entityType
	})).Return(updatedRuleset, nil).Once()
	s.repository.On("DeleteAllRulesetRules", s.ctx, s.transaction, mock.MatchedBy(func(rs models.ScoringRuleset) bool {
		return rs.Version == 2 && rs.EntityType == s.entityType
	})).Return(nil).Once()
	s.repository.On("InsertScoringRulesetVersionRule", s.ctx, s.transaction,
		mock.MatchedBy(func(rs models.ScoringRuleset) bool { return rs.Id == updatedRuleset.Id }),
		mock.MatchedBy(func(r models.CreateScoringRuleRequest) bool {
			return r.StableId == stableId && r.Name == "rule 1"
		})).Return(updatedRule, nil).Once()

	updatedResult, err := s.makeUsecase().CreateRulesetVersion(s.ctx, s.entityType, req)

	s.NoError(err)
	s.Equal(updatedRuleset.Id, updatedResult.Id)
	s.Equal(2, updatedResult.Version)
	s.repository.AssertExpectations(s.T())
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCommitRuleset_SecurityDenied() {
	secErr := fmt.Errorf("forbidden")
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(secErr)

	_, err := s.makeUsecase().CommitRuleset(s.ctx, s.entityType)

	s.ErrorIs(err, secErr)
	s.repository.AssertNotCalled(s.T(), "GetScoringRuleset")
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCommitRuleset_NoDraft() {
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetDraft).
		Return(models.ScoringRuleset{}, models.NotFoundError)

	_, err := s.makeUsecase().CommitRuleset(s.ctx, s.entityType)

	s.ErrorIs(err, models.NotFoundError)
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCommitRuleset_IndexesPending() {
	draft := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Status: string(models.ScoreRulesetDraft)}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetDraft).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{}, 2, nil)

	_, err := s.makeUsecase().CommitRuleset(s.ctx, s.entityType)

	s.ErrorIs(err, models.UnprocessableEntityError)
	s.repository.AssertNotCalled(s.T(), "CommitRuleset")
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCommitRuleset_IndexesNotCreated() {
	draft := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Status: string(models.ScoreRulesetDraft)}
	pendingIndex := models.ConcreteIndex{}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetDraft).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{pendingIndex}, 0, nil)

	_, err := s.makeUsecase().CommitRuleset(s.ctx, s.entityType)

	s.ErrorIs(err, models.UnprocessableEntityError)
	s.repository.AssertNotCalled(s.T(), "CommitRuleset")
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCommitRuleset_HappyPath() {
	draft := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Status: string(models.ScoreRulesetDraft)}
	committed := models.ScoringRuleset{Id: draft.Id, EntityType: s.entityType, Status: string(models.ScoreRulesetCommitted)}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetDraft).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{}, 0, nil)
	s.repository.On("CommitRuleset", s.ctx, s.transaction, draft).Return(committed, nil)

	result, err := s.makeUsecase().CommitRuleset(s.ctx, s.entityType)

	s.NoError(err)
	s.Equal(string(models.ScoreRulesetCommitted), result.Status)
	s.repository.AssertCalled(s.T(), "CommitRuleset", s.ctx, s.transaction, draft)
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_SecurityDenied() {
	secErr := fmt.Errorf("forbidden")
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(secErr)

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.entityType)

	s.ErrorIs(err, secErr)
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_NoDraft() {
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetDraft).
		Return(models.ScoringRuleset{}, models.NotFoundError)

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.entityType)

	s.ErrorIs(err, models.NotFoundError)
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_IndexesPending() {
	draft := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Status: string(models.ScoreRulesetDraft)}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetDraft).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{}, 1, nil) // 1 pending

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.entityType)

	s.ErrorIs(err, models.UnprocessableEntityError)
	s.taskQueue.AssertNotCalled(s.T(), "EnqueueCreateIndexTask")
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_EnqueuesIndexCreation() {
	draft := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Status: string(models.ScoreRulesetDraft)}
	idx := models.ConcreteIndex{}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetDraft).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{idx}, 0, nil)
	s.taskQueue.On("EnqueueCreateIndexTask", s.ctx, s.orgId, []models.ConcreteIndex{idx}).Return(nil)

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.entityType)

	s.NoError(err)
	s.taskQueue.AssertExpectations(s.T())
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_AlreadyReady() {
	draft := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Status: string(models.ScoreRulesetDraft)}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetDraft).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{}, 0, nil) // nothing to do

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.entityType)

	s.NoError(err)
	s.taskQueue.AssertNotCalled(s.T(), "EnqueueCreateIndexTask")
}

func TestScoringRulesetsUsecase(t *testing.T) {
	suite.Run(t, new(ScoringRulesetsUsecaseTestSuite))
}
