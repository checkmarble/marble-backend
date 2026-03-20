package scoring_jobs

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
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
	recordType string
	recordId   string
	ctx        context.Context
}

func (s *TriggeredScoreComputationWorkerTestSuite) SetupTest() {
	s.transaction = new(mocks.Transaction)
	s.transactionFactory = &mocks.TransactionFactory{TxMock: s.transaction}
	s.repository = new(mocks.ScoringRepository)
	s.executorFactory = new(mocks.ExecutorFactory)
	s.dataModelRepo = new(mocks.DataModelRepository)
	s.ingestedDataReader = new(mocks.IngestedDataReader)

	s.orgId = uuid.Must(uuid.NewV7())
	s.recordType = "account"
	s.recordId = "entity-123"
	s.ctx = context.Background()
}

func (s *TriggeredScoreComputationWorkerTestSuite) makeScoreUsecase() scoring.ScoringScoresUsecase {
	return scoring.NewScoringScoresUsecase(
		nil,
		s.executorFactory,
		nil,
		scoring.ScoringRulesetsUsecase{},
		s.repository,
		s.dataModelRepo,
		repositories.OffloadedReadWriter{},
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
			RecordType: s.recordType,
			RecordId:   s.recordId,
		},
	}
}

func (s *TriggeredScoreComputationWorkerTestSuite) makeWorker(
	scoreUsecase scoring.ScoringScoresUsecase,
) *TriggeredScoreComputationWorker {
	return NewTriggeredScoreComputationWorker(
		nil,
		s.transactionFactory,
		scoring.ScoringRulesetsUsecase{},
		scoreUsecase,
		s.repository,
		repositories.OffloadedReadWriter{},
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
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted, 0).
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
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted, 0).
		Return(models.ScoringRuleset{}, repoErr)

	worker := s.makeWorker(scoring.ScoringScoresUsecase{})
	err := worker.Work(s.ctx, s.makeJob())

	s.ErrorIs(err, repoErr)
	s.repository.AssertExpectations(s.T())
}

func (s *TriggeredScoreComputationWorkerTestSuite) TestWork_ScoreIsOverriden() {
	s.T().Setenv(fmt.Sprintf("ENABLE_%s", "USER_SCORING"), s.orgId.String())

	ruleset := models.ScoringRuleset{Id: uuid.Must(uuid.NewV7()), RecordType: s.recordType}
	activeScore := &models.ScoringScore{
		Source: models.ScoreSourceOverride,
	}
	record := models.ScoringRecordRef{OrgId: s.orgId, RecordType: s.recordType, RecordId: s.recordId}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted, 0).
		Return(ruleset, nil)
	s.repository.On("GetActiveScore", s.ctx, s.transaction, record).
		Return(activeScore, nil)

	worker := s.makeWorker(scoring.ScoringScoresUsecase{})
	err := worker.Work(s.ctx, s.makeJob())

	s.NoError(err)
	s.repository.AssertNotCalled(s.T(), "InsertScore")
	s.repository.AssertExpectations(s.T())
}

func (s *TriggeredScoreComputationWorkerTestSuite) TestWork_NoActiveScore_ComputesAndInserts() {
	s.T().Setenv(fmt.Sprintf("ENABLE_%s", "USER_SCORING"), s.orgId.String())

	ruleset := models.ScoringRuleset{Id: uuid.Must(uuid.NewV7()), RecordType: s.recordType, Thresholds: []int{10}}
	record := models.ScoringRecordRef{OrgId: s.orgId, RecordType: s.recordType, RecordId: s.recordId}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			s.recordType: {Name: s.recordType},
		},
	}
	clientExec := new(mocks.Transaction)
	obj := models.DataModelObject{Data: map[string]any{"id": s.recordId}}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted, 0).
		Return(ruleset, nil)
	s.repository.On("GetActiveScore", s.ctx, s.transaction, record).
		Return((*models.ScoringScore)(nil), models.NotFoundError)
	s.dataModelRepo.On("GetDataModel", s.ctx, s.transaction, s.orgId, false, false).
		Return(dataModel, nil)
	s.executorFactory.On("NewClientDbExecutor", s.ctx, s.orgId).Return(clientExec, nil)
	s.ingestedDataReader.On("QueryIngestedObject", s.ctx, clientExec,
		dataModel.Tables[s.recordType], s.recordId, []string(nil)).
		Return([]models.DataModelObject{obj}, nil)
	s.repository.On("InsertScore", s.ctx, s.transaction, mock.MatchedBy(func(r models.InsertScoreRequest) bool {
		return r.OrgId == s.orgId &&
			r.RecordType == s.recordType &&
			r.RecordId == s.recordId &&
			r.Source == models.ScoreSourceRuleset &&
			r.RiskLevel == 1
	})).Return(models.ScoringScore{}, nil)

	worker := s.makeWorker(s.makeScoreUsecase())
	err := worker.Work(s.ctx, s.makeJob())

	s.NoError(err)
	s.repository.AssertExpectations(s.T())
}

func (s *TriggeredScoreComputationWorkerTestSuite) TestWork_ActiveScore_ComputesAndInserts() {
	s.T().Setenv(fmt.Sprintf("ENABLE_%s", "USER_SCORING"), s.orgId.String())

	ruleset := models.ScoringRuleset{Id: uuid.Must(uuid.NewV7()), RecordType: s.recordType, Thresholds: []int{10}}
	record := models.ScoringRecordRef{OrgId: s.orgId, RecordType: s.recordType, RecordId: s.recordId}
	activeScore := &models.ScoringScore{Source: models.ScoreSourceRuleset, RiskLevel: 3}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			s.recordType: {Name: s.recordType},
		},
	}
	clientExec := new(mocks.Transaction)
	obj := models.DataModelObject{Data: map[string]any{"id": s.recordId}}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted, 0).
		Return(ruleset, nil)
	s.repository.On("GetActiveScore", s.ctx, s.transaction, record).
		Return(activeScore, nil)
	s.dataModelRepo.On("GetDataModel", s.ctx, s.transaction, s.orgId, false, false).
		Return(dataModel, nil)
	s.executorFactory.On("NewClientDbExecutor", s.ctx, s.orgId).Return(clientExec, nil)
	s.ingestedDataReader.On("QueryIngestedObject", s.ctx, clientExec,
		dataModel.Tables[s.recordType], s.recordId, []string(nil)).
		Return([]models.DataModelObject{obj}, nil)
	s.repository.On("InsertScore", s.ctx, s.transaction, mock.MatchedBy(func(r models.InsertScoreRequest) bool {
		return r.OrgId == s.orgId &&
			r.RecordType == s.recordType &&
			r.RecordId == s.recordId &&
			r.Source == models.ScoreSourceRuleset &&
			r.RiskLevel == 1
	})).Return(models.ScoringScore{}, nil)

	worker := s.makeWorker(s.makeScoreUsecase())
	err := worker.Work(s.ctx, s.makeJob())

	s.NoError(err)
	s.repository.AssertExpectations(s.T())
}
