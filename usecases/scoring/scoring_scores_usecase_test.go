package scoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func noopAstEvaluationEnvironmentFactory(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
	return ast_eval.AstEvaluationEnvironment{}
}

type TryRefreshScoreTestSuite struct {
	suite.Suite

	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
	executorFactory    *mocks.ExecutorFactory
	repository         *mocks.ScoringRepository
	taskQueue          *mocks.TaskQueueRepository
	enforceSecurity    *mocks.EnforceSecurity
	dataModelRepo      *mocks.DataModelRepository
	ingestedDataReader *mocks.IngestedDataReader

	orgId      uuid.UUID
	recordType string
	recordId   string
	record     models.ScoringRecordRef
	ctx        context.Context
}

func TestTryRefreshScore(t *testing.T) {
	suite.Run(t, new(TryRefreshScoreTestSuite))
}

func (s *TryRefreshScoreTestSuite) SetupTest() {
	s.transaction = new(mocks.Transaction)
	s.transactionFactory = &mocks.TransactionFactory{TxMock: s.transaction}
	s.executorFactory = new(mocks.ExecutorFactory)
	s.repository = new(mocks.ScoringRepository)
	s.taskQueue = new(mocks.TaskQueueRepository)
	s.enforceSecurity = new(mocks.EnforceSecurity)
	s.dataModelRepo = new(mocks.DataModelRepository)
	s.ingestedDataReader = new(mocks.IngestedDataReader)

	s.orgId = uuid.New()
	s.recordType = "account"
	s.recordId = "entity-123"
	s.record = models.ScoringRecordRef{OrgId: s.orgId, RecordType: s.recordType, RecordId: s.recordId}
	s.ctx = context.Background()
}

func (s *TryRefreshScoreTestSuite) makeUsecase() ScoringScoresUsecase {
	return ScoringScoresUsecase{
		enforceSecurity:     s.enforceSecurity,
		executorFactory:     s.executorFactory,
		transactionFactory:  s.transactionFactory,
		repository:          s.repository,
		taskQueueRepository: s.taskQueue,
	}
}

func (s *TryRefreshScoreTestSuite) makeUsecaseWithCompute() ScoringScoresUsecase {
	uc := s.makeUsecase()
	uc.dataModelRepository = s.dataModelRepo
	uc.ingestedDataReader = s.ingestedDataReader
	uc.evaluateAst = ast_eval.EvaluateAstExpression{
		AstEvaluationEnvironmentFactory: noopAstEvaluationEnvironmentFactory,
	}
	return uc
}

func (s *TryRefreshScoreTestSuite) TestTryRefreshScore_BackgroundRefresh_EnqueuesAndReturnsCurrentScore() {
	current := &models.ScoringScore{RiskLevel: 3, Source: models.ScoreSourceRuleset, CreatedAt: time.Now().Add(2 * -time.Hour)}
	opts := models.RefreshScoreOptions{RefreshInBackground: true, RefreshOlderThan: time.Hour}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.taskQueue.On("EnqueueTriggerScoreComputation", s.ctx, s.transaction, s.record).Return(nil)

	uc := s.makeUsecase()
	result, err := uc.tryRefreshScore(s.ctx, current, s.record, opts)

	s.NoError(err)
	s.Equal(current, result)
	s.taskQueue.AssertExpectations(s.T())
}

func (s *TryRefreshScoreTestSuite) TestTryRefreshScore_BackgroundRefresh_EnqueueError() {
	current := &models.ScoringScore{RiskLevel: 3, Source: models.ScoreSourceRuleset, CreatedAt: time.Now().Add(2 * -time.Hour)}
	opts := models.RefreshScoreOptions{RefreshInBackground: true, RefreshOlderThan: time.Hour}
	enqueueErr := fmt.Errorf("queue full")

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.taskQueue.On("EnqueueTriggerScoreComputation", s.ctx, s.transaction, s.record).Return(enqueueErr)

	uc := s.makeUsecase()
	_, err := uc.tryRefreshScore(s.ctx, current, s.record, opts)

	s.ErrorIs(err, enqueueErr)
}

func (s *TryRefreshScoreTestSuite) TestTryRefreshScore_NoScore_BackgroundFallsThrough() {
	opts := models.RefreshScoreOptions{RefreshInBackground: true, RefreshOlderThan: time.Hour}
	computeErr := fmt.Errorf("no ruleset")

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted).
		Return(models.ScoringRuleset{}, computeErr)

	uc := s.makeUsecase()
	_, err := uc.tryRefreshScore(s.ctx, nil, s.record, opts)

	s.ErrorIs(err, computeErr)
	s.taskQueue.AssertNotCalled(s.T(), "EnqueueTriggerScoreComputation")
}

func (s *TryRefreshScoreTestSuite) TestTryRefreshScore_NotStale() {
	current := &models.ScoringScore{
		RiskLevel: 3,
		Source:    models.ScoreSourceRuleset,
		CreatedAt: time.Now().Add(-time.Minute),
	}
	opts := models.RefreshScoreOptions{RefreshOlderThan: time.Hour}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)

	uc := s.makeUsecase()
	result, err := uc.tryRefreshScore(s.ctx, current, s.record, opts)

	s.NoError(err)
	s.Equal(current, result)
	s.repository.AssertNotCalled(s.T(), "GetScoringRuleset")
	s.taskQueue.AssertNotCalled(s.T(), "EnqueueTriggerScoreComputation")
}

func (s *TryRefreshScoreTestSuite) TestTryRefreshScore_Stale_ComputeError_HasCurrentScore() {
	current := &models.ScoringScore{
		RiskLevel: 3,
		Source:    models.ScoreSourceRuleset,
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	opts := models.RefreshScoreOptions{RefreshOlderThan: time.Hour}
	computeErr := fmt.Errorf("compute failed")

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted).
		Return(models.ScoringRuleset{}, computeErr)

	uc := s.makeUsecase()
	result, err := uc.tryRefreshScore(s.ctx, current, s.record, opts)

	s.NoError(err)
	s.Equal(current, result)
}

func (s *TryRefreshScoreTestSuite) TestTryRefreshScore_Stale_ComputeError_NoCurrentScore() {
	opts := models.RefreshScoreOptions{RefreshOlderThan: time.Hour}
	computeErr := fmt.Errorf("compute failed")

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted).
		Return(models.ScoringRuleset{}, computeErr)

	uc := s.makeUsecase()
	_, err := uc.tryRefreshScore(s.ctx, nil, s.record, opts)

	s.ErrorIs(err, computeErr)
}

func (s *TryRefreshScoreTestSuite) TestTryRefreshScore_Stale_ComputeAndInsert_HappyPath() {
	current := &models.ScoringScore{
		RiskLevel: 2,
		Source:    models.ScoreSourceRuleset,
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	opts := models.RefreshScoreOptions{RefreshOlderThan: time.Hour}

	ruleset := models.ScoringRuleset{
		Id:         uuid.New(),
		RecordType: s.recordType,
		Thresholds: []int{10},
	}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			s.recordType: {Name: s.recordType},
		},
	}
	clientExec := new(mocks.Transaction)
	obj := models.DataModelObject{Data: map[string]any{"id": s.recordId}}
	inserted := models.ScoringScore{RiskLevel: 2, Source: models.ScoreSourceRuleset}

	expectedReq := models.InsertScoreRequest{
		OrgId:      s.orgId,
		RecordType: s.recordType,
		RecordId:   s.recordId,
		RiskLevel:  1,
		Source:     models.ScoreSourceRuleset,
		RulesetId:  &ruleset.Id,
	}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.enforceSecurity.On("ReadOrganization", mock.Anything).Return(nil)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted).
		Return(ruleset, nil)
	s.dataModelRepo.On("GetDataModel", s.ctx, s.transaction, s.orgId, false, false).
		Return(dataModel, nil)
	s.executorFactory.On("NewClientDbExecutor", s.ctx, s.orgId).Return(clientExec, nil)
	s.ingestedDataReader.On("QueryIngestedObject", s.ctx, clientExec, dataModel.Tables[s.recordType], s.recordId, []string(nil)).
		Return([]models.DataModelObject{obj}, nil)
	s.repository.On("InsertScore", s.ctx, s.transaction, expectedReq).
		Return(inserted, nil)

	uc := s.makeUsecaseWithCompute()
	result, err := uc.tryRefreshScore(s.ctx, current, s.record, opts)

	s.NoError(err)
	s.Equal(&inserted, result)
	s.repository.AssertExpectations(s.T())
}

func (s *TryRefreshScoreTestSuite) TestTryRefreshScore_Stale_InsertError_HasCurrentScore() {
	current := &models.ScoringScore{
		RiskLevel: 1,
		Source:    models.ScoreSourceRuleset,
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	opts := models.RefreshScoreOptions{RefreshOlderThan: time.Hour}
	insertErr := fmt.Errorf("insert failed")

	ruleset := models.ScoringRuleset{
		Id:         uuid.New(),
		RecordType: s.recordType,
		Thresholds: []int{10},
	}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			s.recordType: {Name: s.recordType},
		},
	}
	clientExec := new(mocks.Transaction)
	obj := models.DataModelObject{Data: map[string]any{"id": s.recordId}}

	s.transactionFactory.On("Transaction", s.ctx, mock.Anything).Return(nil)
	s.enforceSecurity.On("OrgId").Return(s.orgId)
	s.enforceSecurity.On("ReadOrganization", mock.Anything).Return(nil)
	s.executorFactory.On("NewExecutor").Return(s.transaction)
	s.repository.On("GetScoringRuleset", s.ctx, s.transaction, s.orgId, s.recordType, models.ScoreRulesetCommitted).
		Return(ruleset, nil)
	s.dataModelRepo.On("GetDataModel", s.ctx, s.transaction, s.orgId, false, false).
		Return(dataModel, nil)
	s.executorFactory.On("NewClientDbExecutor", s.ctx, s.orgId).Return(clientExec, nil)
	s.ingestedDataReader.On("QueryIngestedObject", s.ctx, clientExec, dataModel.Tables[s.recordType], s.recordId, []string(nil)).
		Return([]models.DataModelObject{obj}, nil)
	s.repository.On("InsertScore", s.ctx, s.transaction, mock.Anything).Return(models.ScoringScore{}, insertErr)

	uc := s.makeUsecaseWithCompute()
	result, err := uc.tryRefreshScore(s.ctx, current, s.record, opts)

	s.NoError(err)
	s.Equal(current, result)
}
