package scoring

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/dto"
	dtoScoring "github.com/checkmarble/marble-backend/dto/scoring"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func validateScoringAst(ctx context.Context, nodeDto dto.NodeDto) error {
	orgId := uuid.New()

	sec := &security.EnforceSecurityScoringImpl{
		EnforceSecurity: &security.EnforceSecurityImpl{
			Credentials: models.Credentials{OrganizationId: orgId},
		},
	}

	exec := new(mocks.Executor)
	execFactory := new(mocks.ExecutorFactory)
	dataModelRepository := new(mocks.DataModelRepository)
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"any": {},
		},
	}

	execFactory.On("NewExecutor").Return(exec)
	dataModelRepository.On("GetDataModel", mock.Anything, exec, orgId, false, false).Return(dataModel, nil)

	uc := ScoringRulesetsUsecase{
		enforceSecurity: sec,
		validateScenarioAst: &scenarios.ValidateScenarioAstImpl{
			AstValidator: &scenarios.AstValidatorImpl{
				ExecutorFactory:     execFactory,
				DataModelRepository: dataModelRepository,
				AstEvaluationEnvironmentFactory: func(p ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
					return ast_eval.NewAstEvaluationEnvironment()
				},
			},
		},
	}

	node, _ := dto.AdaptASTNode(nodeDto)

	if validation, _ := uc.ValidateAst(ctx, "any", &node); len(validation.Errors) > 0 {
		return errors.Join(pure_utils.Map(validation.Errors, func(e models.ScenarioValidationError) error { return e.Error })...)
	}
	if err := uc.validateScoringRuleAst(nodeDto); err != nil {
		return err
	}

	return nil
}

func TestValidateAst_ScoreComputation(t *testing.T) {
	tree := dto.NodeDto{
		Name:     "ScoreComputation",
		Children: []dto.NodeDto{{Constant: true}},
		NamedChildren: map[string]dto.NodeDto{
			"modifier": {Constant: 1.0},
		},
	}

	assert.NoError(t, validateScoringAst(t.Context(), tree))
}

func TestValidateAst_InvalidRoot(t *testing.T) {
	tree := dto.NodeDto{Constant: 42}
	err := validateScoringAst(t.Context(), tree)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ast node does not return a [score_computation_result]")
}

func TestValidateAst_Switch_WithScoreComputationChildren(t *testing.T) {
	tree := dto.NodeDto{
		Name: "Switch",
		Children: []dto.NodeDto{
			{Name: "ScoreComputation", Children: []dto.NodeDto{{Constant: true}}, NamedChildren: map[string]dto.NodeDto{"modifier": {Constant: 2}}},
			{Name: "ScoreComputation", Children: []dto.NodeDto{{Constant: true}}, NamedChildren: map[string]dto.NodeDto{"modifier": {Constant: 3}}},
		},
		NamedChildren: map[string]dto.NodeDto{
			"field": {Constant: "Hello, world"},
		},
	}

	err := validateScoringAst(t.Context(), tree)

	require.NoError(t, err)
}

func TestValidateAst_Switch_WrongChildType(t *testing.T) {
	tree := dto.NodeDto{
		Name: "Switch",
		Children: []dto.NodeDto{
			{Name: "And"},
		},
		NamedChildren: map[string]dto.NodeDto{
			"field": {Constant: "Hello, world"},
		},
	}

	err := validateScoringAst(t.Context(), tree)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ast node does not return a [score_computation_result]")
}

type ScoringRulesetsUsecaseTestSuite struct {
	suite.Suite

	transaction         *mocks.Transaction
	transactionFactory  *mocks.TransactionFactory
	executorFactory     *mocks.ExecutorFactory
	repository          *mocks.ScoringRepository
	dataModelRepository *mocks.DataModelRepository
	taskQueue           *mocks.TaskQueueRepository
	indexEditor         *mocks.ClientDbIndexEditor
	enforceSecurity     *mocks.EnforceSecurity

	orgId      uuid.UUID
	recordType string
	ctx        context.Context
}

func (s *ScoringRulesetsUsecaseTestSuite) SetupTest() {
	s.transaction = new(mocks.Transaction)
	s.transactionFactory = &mocks.TransactionFactory{TxMock: s.transaction}
	s.executorFactory = new(mocks.ExecutorFactory)
	s.repository = new(mocks.ScoringRepository)
	s.dataModelRepository = new(mocks.DataModelRepository)
	s.taskQueue = new(mocks.TaskQueueRepository)
	s.indexEditor = new(mocks.ClientDbIndexEditor)
	s.enforceSecurity = new(mocks.EnforceSecurity)
	s.orgId = pure_utils.NewId()

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"account": {},
		},
	}

	s.recordType = "account"
	s.ctx = context.Background()
	s.dataModelRepository.On("GetDataModel", mock.Anything, mock.Anything, s.orgId, false, false).Return(dataModel, nil)
}

func (s *ScoringRulesetsUsecaseTestSuite) makeUsecase() ScoringRulesetsUsecase {
	return ScoringRulesetsUsecase{
		enforceSecurity:     s.enforceSecurity,
		executorFactory:     s.executorFactory,
		transactionFactory:  s.transactionFactory,
		repository:          s.repository,
		indexEditor:         s.indexEditor,
		taskQueueRepository: s.taskQueue,
		validateScenarioAst: &scenarios.ValidateScenarioAstImpl{
			AstValidator: &scenarios.AstValidatorImpl{
				ExecutorFactory:     s.executorFactory,
				DataModelRepository: s.dataModelRepository,
				AstEvaluationEnvironmentFactory: func(p ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
					return ast_eval.NewAstEvaluationEnvironment()
				},
			},
		},
	}
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCreateRulesetVersion_InsertsRulesetAndRules() {
	stableId := pure_utils.NewId()
	req := dtoScoring.CreateRulesetRequest{
		Name:        "my ruleset",
		Description: "desc",
		Thresholds:  []int{10, 20},
		Rules: []dtoScoring.CreateRuleRequest{
			{
				StableId:    stableId,
				Name:        "rule 1",
				RiskType:    "customer_features",
				Description: "rule desc",
				Ast: dto.NodeDto{
					Name:          "ScoreComputation",
					Children:      []dto.NodeDto{{Constant: true}},
					NamedChildren: map[string]dto.NodeDto{"modifier": {Constant: 2}},
				},
			},
		},
	}

	insertedRuleset := models.ScoringRuleset{Id: pure_utils.NewId(), RecordType: s.recordType, Version: 1}
	insertedRule := models.ScoringRule{Id: pure_utils.NewId(), StableId: stableId, Name: "rule 1"}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringSettings", s.ctx, s.transaction, s.orgId).
		Return(&models.ScoringSettings{MaxRiskLevel: 3}, nil).Once()
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted, 0).
		Return(models.ScoringRuleset{}, models.NotFoundError).Once()
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("InsertScoringRulesetVersion", s.ctx, s.transaction, s.orgId, mock.MatchedBy(func(
		r models.CreateScoringRulesetRequest,
	) bool {
		return r.Version == 1 && r.Name == req.Name && r.RecordType == s.recordType
	})).Return(insertedRuleset, nil).Once()
	s.repository.On("InsertScoringRulesetVersionRule", s.ctx, s.transaction,
		mock.MatchedBy(func(rs models.ScoringRuleset) bool { return rs.Id == insertedRuleset.Id }),
		mock.MatchedBy(func(r []models.CreateScoringRuleRequest) bool {
			return len(r) == 1 && r[0].StableId == stableId && r[0].Name == "rule 1"
		})).Return([]models.ScoringRule{insertedRule}, nil).Once()

	result, err := s.makeUsecase().CreateRulesetVersion(s.ctx, s.recordType, req)

	s.NoError(err)
	s.Equal(insertedRuleset.Id, result.Id)
	s.Equal(1, result.Version)

	updatedRuleset := models.ScoringRuleset{Id: pure_utils.NewId(), RecordType: s.recordType, Version: 2}
	updatedRule := models.ScoringRule{Id: pure_utils.NewId(), StableId: stableId, Name: "rule 1"}

	s.repository.On("GetScoringSettings", s.ctx, s.transaction, s.orgId).
		Return(&models.ScoringSettings{MaxRiskLevel: 3}, nil).Once()
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted, 0).
		Return(models.ScoringRuleset{Id: insertedRuleset.Id, Version: 1}, nil).Once()
	s.repository.On("InsertScoringRulesetVersion", s.ctx, s.transaction, s.orgId, mock.MatchedBy(func(
		r models.CreateScoringRulesetRequest,
	) bool {
		return r.Version == 2 && r.Name == req.Name && r.RecordType == s.recordType
	})).Return(updatedRuleset, nil).Once()
	s.repository.On("CancelRulesetDryRun", s.ctx, s.transaction,
		mock.MatchedBy(func(rs models.ScoringRuleset) bool {
			return rs.Id == insertedRuleset.Id
		})).Return(nil)
	s.repository.On("InsertScoringRulesetVersionRule", s.ctx, s.transaction,
		mock.MatchedBy(func(rs models.ScoringRuleset) bool { return rs.Id == updatedRuleset.Id }),
		mock.MatchedBy(func(r []models.CreateScoringRuleRequest) bool {
			return len(r) == 1 && r[0].StableId == stableId && r[0].Name == "rule 1"
		})).Return([]models.ScoringRule{updatedRule}, nil).Once()

	updatedResult, err := s.makeUsecase().CreateRulesetVersion(s.ctx, s.recordType, req)

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

	_, err := s.makeUsecase().CommitRuleset(s.ctx, s.recordType)

	s.ErrorIs(err, secErr)
	s.repository.AssertNotCalled(s.T(), "GetScoringRuleset")
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCommitRuleset_NoDraft() {
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetDraft, 0).
		Return(models.ScoringRuleset{}, models.NotFoundError)

	_, err := s.makeUsecase().CommitRuleset(s.ctx, s.recordType)

	s.ErrorIs(err, models.NotFoundError)
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCommitRuleset_IndexesPending() {
	draft := models.ScoringRuleset{Id: pure_utils.NewId(), RecordType: s.recordType, Status: models.ScoreRulesetDraft}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetDraft, 0).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{}, 2, nil)

	_, err := s.makeUsecase().CommitRuleset(s.ctx, s.recordType)

	s.ErrorIs(err, models.UnprocessableEntityError)
	s.repository.AssertNotCalled(s.T(), "CommitRuleset")
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCommitRuleset_IndexesNotCreated() {
	draft := models.ScoringRuleset{Id: pure_utils.NewId(), RecordType: s.recordType, Status: models.ScoreRulesetDraft}
	pendingIndex := models.ConcreteIndex{}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetDraft, 0).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{pendingIndex}, 0, nil)

	_, err := s.makeUsecase().CommitRuleset(s.ctx, s.recordType)

	s.ErrorIs(err, models.UnprocessableEntityError)
	s.repository.AssertNotCalled(s.T(), "CommitRuleset")
}

func (s *ScoringRulesetsUsecaseTestSuite) TestCommitRuleset_HappyPath() {
	draft := models.ScoringRuleset{Id: pure_utils.NewId(), RecordType: s.recordType, Status: models.ScoreRulesetDraft}
	committed := models.ScoringRuleset{Id: draft.Id, RecordType: s.recordType, Status: models.ScoreRulesetCommitted}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetDraft, 0).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{}, 0, nil)
	s.repository.On("CancelRulesetDryRun", s.ctx, s.transaction, draft).Return(nil)
	s.repository.On("CommitRuleset", s.ctx, s.transaction, draft).Return(committed, nil)

	result, err := s.makeUsecase().CommitRuleset(s.ctx, s.recordType)

	s.NoError(err)
	s.Equal(models.ScoreRulesetCommitted, result.Status)
	s.repository.AssertCalled(s.T(), "CommitRuleset", s.ctx, s.transaction, draft)
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_SecurityDenied() {
	secErr := fmt.Errorf("forbidden")
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(secErr)

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.recordType)

	s.ErrorIs(err, secErr)
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_NoDraft() {
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetDraft, 0).
		Return(models.ScoringRuleset{}, models.NotFoundError)

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.recordType)

	s.ErrorIs(err, models.NotFoundError)
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_IndexesPending() {
	draft := models.ScoringRuleset{Id: pure_utils.NewId(), RecordType: s.recordType, Status: models.ScoreRulesetDraft}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetDraft, 0).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{}, 1, nil) // 1 pending

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.recordType)

	s.ErrorIs(err, models.UnprocessableEntityError)
	s.taskQueue.AssertNotCalled(s.T(), "EnqueueCreateIndexTask")
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_EnqueuesIndexCreation() {
	draft := models.ScoringRuleset{Id: pure_utils.NewId(), RecordType: s.recordType, Status: models.ScoreRulesetDraft}
	idx := models.ConcreteIndex{}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetDraft, 0).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{idx}, 0, nil)
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.taskQueue.On("EnqueueCreateIndexTask", s.ctx, s.transaction, s.orgId, []models.ConcreteIndex{idx}).Return(nil)

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.recordType)

	s.NoError(err)
	s.taskQueue.AssertExpectations(s.T())
}

func (s *ScoringRulesetsUsecaseTestSuite) TestPrepareRuleset_AlreadyReady() {
	draft := models.ScoringRuleset{Id: pure_utils.NewId(), RecordType: s.recordType, Status: models.ScoreRulesetDraft}

	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.enforceSecurity.On("UpdateRuleset", s.orgId).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetDraft, 0).
		Return(draft, nil)
	s.indexEditor.On("GetIndexesToCreateForScoringRuleset", s.ctx, s.orgId, draft).
		Return([]models.ConcreteIndex{}, 0, nil) // nothing to do

	err := s.makeUsecase().PrepareRuleset(s.ctx, s.recordType)

	s.NoError(err)
	s.taskQueue.AssertNotCalled(s.T(), "EnqueueCreateIndexTask")
}

func TestScoringRulesetsUsecase(t *testing.T) {
	suite.Run(t, new(ScoringRulesetsUsecaseTestSuite))
}
