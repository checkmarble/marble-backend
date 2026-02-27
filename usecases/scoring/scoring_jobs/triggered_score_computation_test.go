package scoring_jobs

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/scoring"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func noopAstEvaluationEnvironmentFactory(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
	return ast_eval.AstEvaluationEnvironment{}
}

func TestTriggeredScoreComputationWorker(t *testing.T) {
	suite.Run(t, new(TriggeredScoreComputationWorkerTestSuite))
}

// TriggeredScoreComputationWorkerTestSuite tests the Work method branching logic.
type TriggeredScoreComputationWorkerTestSuite struct {
	suite.Suite

	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
	repository         *mocks.ScoringRepository
	executorFactory    *mocks.ExecutorFactory
	dataModelRepo      *mocks.DataModelRepository
	ingestedDataReader *mocks.IngestedDataReader

	orgId      uuid.UUID
	entityType string
	entityId   string
	ctx        context.Context
}

func (s *TriggeredScoreComputationWorkerTestSuite) SetupTest() {
	s.transaction = new(mocks.Transaction)
	s.transactionFactory = &mocks.TransactionFactory{TxMock: s.transaction}
	s.repository = new(mocks.ScoringRepository)
	s.executorFactory = new(mocks.ExecutorFactory)
	s.dataModelRepo = new(mocks.DataModelRepository)
	s.ingestedDataReader = new(mocks.IngestedDataReader)

	s.orgId = uuid.New()
	s.entityType = "account"
	s.entityId = "entity-123"
	s.ctx = context.Background()
}

func (s *TriggeredScoreComputationWorkerTestSuite) makeScoreUsecase() scoring.ScoringScoresUsecase {
	return scoring.NewScoringScoresUsecase(
		nil,
		s.executorFactory,
		nil,
		s.repository,
		s.dataModelRepo,
		s.ingestedDataReader,
		nil,
		ast_eval.EvaluateAstExpression{
			AstEvaluationEnvironmentFactory: noopAstEvaluationEnvironmentFactory,
		},
	)
}

func (s *TriggeredScoreComputationWorkerTestSuite) makeJob() *river.Job[models.TriggeredScoreComputationArgs] {
	return &river.Job[models.TriggeredScoreComputationArgs]{
		Args: models.TriggeredScoreComputationArgs{
			OrgId:      s.orgId,
			EntityType: s.entityType,
			EntityId:   s.entityId,
		},
	}
}

func (s *TriggeredScoreComputationWorkerTestSuite) makeWorker(scoreUsecase scoring.ScoringScoresUsecase) *TriggeredScoreComputationWorker {
	return NewTriggeredScoreComputationWorker(
		nil,
		s.transactionFactory,
		scoring.ScoringRulesetsUsecase{},
		scoreUsecase,
		s.repository,
	)
}

func (s *TriggeredScoreComputationWorkerTestSuite) TestWork_FeatureFlagDisabled() {
	s.T().Setenv(fmt.Sprintf("ENABLE_%s", "USER_SCORING"), "")

	worker := s.makeWorker(scoring.ScoringScoresUsecase{})
	err := worker.Work(s.ctx, s.makeJob())

	s.NoError(err)
	s.repository.AssertNotCalled(s.T(), "GetScoringRuleset")
}

func (s *TriggeredScoreComputationWorkerTestSuite) TestWork_NoRuleset() {
	s.T().Setenv(fmt.Sprintf("ENABLE_%s", "USER_SCORING"), s.orgId.String())

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetCommitted).
		Return(models.ScoringRuleset{}, models.NotFoundError)

	worker := s.makeWorker(scoring.ScoringScoresUsecase{})
	err := worker.Work(s.ctx, s.makeJob())

	s.NoError(err)
	s.repository.AssertNotCalled(s.T(), "InsertScore")
	s.repository.AssertExpectations(s.T())
}

func (s *TriggeredScoreComputationWorkerTestSuite) TestWork_RepoError() {
	s.T().Setenv(fmt.Sprintf("ENABLE_%s", "USER_SCORING"), s.orgId.String())

	repoErr := fmt.Errorf("db connection lost")
	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetCommitted).
		Return(models.ScoringRuleset{}, repoErr)

	worker := s.makeWorker(scoring.ScoringScoresUsecase{})
	err := worker.Work(s.ctx, s.makeJob())

	s.ErrorIs(err, repoErr)
	s.repository.AssertExpectations(s.T())
}

func (s *TriggeredScoreComputationWorkerTestSuite) TestWork_ScoreIsOverriden() {
	s.T().Setenv(fmt.Sprintf("ENABLE_%s", "USER_SCORING"), s.orgId.String())

	ruleset := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType}
	activeScore := &models.ScoringScore{
		Source: models.ScoreSourceOverride,
	}
	entityRef := models.ScoringEntityRef{OrgId: s.orgId, EntityType: s.entityType, EntityId: s.entityId}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetCommitted).
		Return(ruleset, nil)
	s.repository.On("GetActiveScore", s.ctx, s.transaction, entityRef).
		Return(activeScore, nil)

	worker := s.makeWorker(scoring.ScoringScoresUsecase{})
	err := worker.Work(s.ctx, s.makeJob())

	s.NoError(err)
	s.repository.AssertNotCalled(s.T(), "InsertScore")
	s.repository.AssertExpectations(s.T())
}

func (s *TriggeredScoreComputationWorkerTestSuite) TestWork_NoActiveScore_ComputesAndInserts() {
	s.T().Setenv(fmt.Sprintf("ENABLE_%s", "USER_SCORING"), s.orgId.String())

	ruleset := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Thresholds: []int{10}}
	entityRef := models.ScoringEntityRef{OrgId: s.orgId, EntityType: s.entityType, EntityId: s.entityId}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			s.entityType: {Name: s.entityType},
		},
	}
	clientExec := new(mocks.Transaction)
	obj := models.DataModelObject{Data: map[string]any{"id": s.entityId}}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetCommitted).
		Return(ruleset, nil)
	s.repository.On("GetActiveScore", s.ctx, s.transaction, entityRef).
		Return((*models.ScoringScore)(nil), models.NotFoundError)
	s.dataModelRepo.On("GetDataModel", s.ctx, s.transaction, s.orgId, false, false).
		Return(dataModel, nil)
	s.executorFactory.On("NewClientDbExecutor", s.ctx, s.orgId).Return(clientExec, nil)
	s.ingestedDataReader.On("QueryIngestedObject", s.ctx, clientExec, dataModel.Tables[s.entityType], s.entityId, []string(nil)).
		Return([]models.DataModelObject{obj}, nil)
	s.repository.On("InsertScore", s.ctx, s.transaction, mock.MatchedBy(func(r models.InsertScoreRequest) bool {
		return r.OrgId == s.orgId &&
			r.EntityType == s.entityType &&
			r.EntityId == s.entityId &&
			r.Source == models.ScoreSourceRuleset &&
			r.Score == 1
	})).Return(models.ScoringScore{}, nil)

	worker := s.makeWorker(s.makeScoreUsecase())
	err := worker.Work(s.ctx, s.makeJob())

	s.NoError(err)
	s.repository.AssertExpectations(s.T())
}

func (s *TriggeredScoreComputationWorkerTestSuite) TestWork_ActiveScore_ComputesAndInserts() {
	s.T().Setenv(fmt.Sprintf("ENABLE_%s", "USER_SCORING"), s.orgId.String())

	ruleset := models.ScoringRuleset{Id: uuid.New(), EntityType: s.entityType, Thresholds: []int{10}}
	entityRef := models.ScoringEntityRef{OrgId: s.orgId, EntityType: s.entityType, EntityId: s.entityId}
	activeScore := &models.ScoringScore{Source: models.ScoreSourceRuleset, Score: 3}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			s.entityType: {Name: s.entityType},
		},
	}
	clientExec := new(mocks.Transaction)
	obj := models.DataModelObject{Data: map[string]any{"id": s.entityId}}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.entityType, models.ScoreRulesetCommitted).
		Return(ruleset, nil)
	s.repository.On("GetActiveScore", s.ctx, s.transaction, entityRef).
		Return(activeScore, nil)
	s.dataModelRepo.On("GetDataModel", s.ctx, s.transaction, s.orgId, false, false).
		Return(dataModel, nil)
	s.executorFactory.On("NewClientDbExecutor", s.ctx, s.orgId).Return(clientExec, nil)
	s.ingestedDataReader.On("QueryIngestedObject", s.ctx, clientExec, dataModel.Tables[s.entityType], s.entityId, []string(nil)).
		Return([]models.DataModelObject{obj}, nil)
	s.repository.On("InsertScore", s.ctx, s.transaction, mock.MatchedBy(func(r models.InsertScoreRequest) bool {
		return r.OrgId == s.orgId &&
			r.EntityType == s.entityType &&
			r.EntityId == s.entityId &&
			r.Source == models.ScoreSourceRuleset &&
			r.Score == 1
	})).Return(models.ScoringScore{}, nil)

	worker := s.makeWorker(s.makeScoreUsecase())
	err := worker.Work(s.ctx, s.makeJob())

	s.NoError(err)
	s.repository.AssertExpectations(s.T())
}
