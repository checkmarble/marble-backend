package usecases

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/feature_access"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/scoring"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CaseEntityReaderSuite struct {
	suite.Suite
	ctx      context.Context
	org      uuid.UUID
	model    *mocks.DataModelRepository
	security *mocks.EnforceSecurity
	factory  *mocks.ExecutorFactory
	objects  *mocks.IngestedDataReader
	indexes  *mocks.ClientDbIndexEditor
	exec     *mocks.Executor
	reader   caseEntityReader
	caseRepo *mocks.CaseRepository
	feature  *mocks.FeatureAccessReader
	queue    *mocks.TaskQueueRepository
	tx       *mocks.Transaction
}

func (s *CaseEntityReaderSuite) SetupTest() {
	s.ctx = context.Background()
	s.org = uuid.New()
	s.model = new(mocks.DataModelRepository)
	s.security = new(mocks.EnforceSecurity)
	s.factory = new(mocks.ExecutorFactory)
	s.objects = new(mocks.IngestedDataReader)
	s.indexes = new(mocks.ClientDbIndexEditor)
	s.exec = new(mocks.Executor)
	s.caseRepo = new(mocks.CaseRepository)
	s.feature = new(mocks.FeatureAccessReader)
	s.queue = new(mocks.TaskQueueRepository)
	s.tx = new(mocks.Transaction)
	dm := usecase{dataModelRepository: s.model, enforceSecurity: s.security, executorFactory: s.factory, clientDbIndexEditor: s.indexes}
	s.reader = caseEntityReader{dataModelUsecase: dm, clientDbRepository: s.objects, executorFactory: s.factory}
}
func (s *CaseEntityReaderSuite) TearDownTest() {
	s.model.AssertExpectations(s.T())
	s.security.AssertExpectations(s.T())
	s.factory.AssertExpectations(s.T())
	s.objects.AssertExpectations(s.T())
	s.indexes.AssertExpectations(s.T())
	s.caseRepo.AssertExpectations(s.T())
	s.feature.AssertExpectations(s.T())
	s.queue.AssertExpectations(s.T())
	s.tx.AssertExpectations(s.T())
}
func (s *CaseEntityReaderSuite) expectModel(dm models.DataModel) {
	s.factory.On("NewExecutor").Return(s.exec).Once()
	s.model.On("GetDataModel", s.ctx, s.exec, s.org, false, true).Return(dm, nil).Once()
}
func (s *CaseEntityReaderSuite) SetupSubTest()    { s.SetupTest() }
func (s *CaseEntityReaderSuite) TearDownSubTest() { s.TearDownTest() }

func (s *CaseEntityReaderSuite) makeCaseUsecase() CaseUseCase {
	return CaseUseCase{
		repository:          s.caseRepo,
		caseEntityReader:    s.reader,
		featureAccessReader: s.feature,
		taskQueueRepository: s.queue,
	}
}

func (s *CaseEntityReaderSuite) TestEligibility() {
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
	tests := []struct {
		name     string
		semantic models.SemanticType
		eligible bool
	}{
		{name: "person", semantic: models.SemanticTypePerson, eligible: true},
		{name: "company", semantic: models.SemanticTypeCompany, eligible: true},
		{name: "partner", semantic: models.SemanticTypePartner, eligible: true},
		{name: "account", semantic: models.SemanticTypeAccount},
		{name: "transaction", semantic: models.SemanticTypeTransaction},
		{name: "other", semantic: models.SemanticTypeOther},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			table := models.Table{Name: ref.TableName, SemanticType: tt.semantic}
			s.expectModel(models.DataModel{Tables: map[string]models.Table{ref.TableName: table}})
			if tt.eligible {
				s.factory.On("NewClientDbExecutor", s.ctx, s.org).Return(s.exec, nil).Once()
				s.objects.On("QueryIngestedObjectByUniqueField", s.ctx, s.exec, table, ref.ObjectId, "object_id", []string(nil)).Return([]models.DataModelObject{{Data: map[string]any{"object_id": ref.ObjectId}}}, nil).Once()
				s.NoError(s.reader.RequireActiveCaseEntity(s.ctx, s.org, ref))
			} else {
				s.ErrorIs(s.reader.RequireActiveCaseEntity(s.ctx, s.org, ref), models.UnprocessableEntityError)
				s.factory.AssertNotCalled(s.T(), "NewClientDbExecutor", mock.Anything, mock.Anything)
				s.objects.AssertNotCalled(s.T(), "QueryIngestedObjectByUniqueField", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}
func (s *CaseEntityReaderSuite) TestManualLinksProjection() {
	s.Run("empty links", func() {
		uc := CaseUseCase{caseEntityReader: s.reader}

		entities, err := uc.readCaseEntities(s.ctx, s.org, nil)

		s.NoError(err)
		s.NotNil(entities)
		s.Empty(entities)
		s.factory.AssertNotCalled(s.T(), "NewExecutor")
	})
	s.Run("customer data and missing historical object", func() {
		table := models.Table{Name: "customers"}
		known := models.CaseEntityRef{TableName: table.Name, ObjectId: "c-123"}
		missing := models.CaseEntityRef{TableName: table.Name, ObjectId: "c-456"}
		s.expectModel(models.DataModel{Tables: map[string]models.Table{table.Name: table}})
		s.factory.On("NewClientDbExecutor", s.ctx, s.org).Return(s.exec, nil).Once()
		ids := mock.MatchedBy(func(ids []string) bool {
			return len(ids) == 2 && ((ids[0] == known.ObjectId && ids[1] == missing.ObjectId) || (ids[1] == known.ObjectId && ids[0] == missing.ObjectId))
		})
		s.objects.On("QueryIngestedObjectsByIds", s.ctx, s.exec, table, ids, []string(nil)).Return([]models.DataModelObject{{Data: map[string]any{"object_id": known.ObjectId, "name": "Alice"}}}, nil).Once()
		uc := CaseUseCase{caseEntityReader: s.reader}

		entities, err := uc.readCaseEntities(s.ctx, s.org, []models.CaseManualEntity{{CaseEntityRef: missing}, {CaseEntityRef: known}})

		s.NoError(err)
		s.Require().Len(entities, 2)
		s.Equal(known, entities[0].CaseEntityRef)
		s.Equal("Alice", entities[0].Data["name"])
		s.Equal(missing, entities[1].CaseEntityRef)
		s.Nil(entities[1].Data)
		s.model.AssertNotCalled(s.T(), "ListPivots", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestCaseEntityReaderSuite(t *testing.T) { suite.Run(t, new(CaseEntityReaderSuite)) }

func (s *CaseEntityReaderSuite) TestLinkChangesDoNotRefreshScores() {
	tests := []struct {
		name    string
		add     bool
		changed bool
	}{
		{name: "addition", add: true, changed: true},
		{name: "removal", changed: true},
		{name: "duplicate addition", add: true},
		{name: "missing removal"},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
			var link *models.CaseManualEntity
			if tt.changed {
				link = &models.CaseManualEntity{Id: uuid.NewString(), CaseEntityRef: ref}
			}
			method := "DeleteCaseManualEntity"
			if tt.add {
				method = "InsertCaseManualEntity"
			}
			s.caseRepo.On(method, s.ctx, s.tx, s.org, "case", ref).Return(link, nil).Once()
			if tt.changed {
				if tt.add {
					table := models.Table{Name: ref.TableName, SemanticType: models.SemanticTypePerson}
					s.expectModel(models.DataModel{Tables: map[string]models.Table{table.Name: table}})
					s.factory.On("NewClientDbExecutor", s.ctx, s.org).Return(s.exec, nil).Once()
					s.objects.On("QueryIngestedObjectByUniqueField", s.ctx, s.exec, table, ref.ObjectId, "object_id", []string(nil)).Return([]models.DataModelObject{{}}, nil).Once()
				}
				s.caseRepo.On("CreateCaseEvent", s.ctx, s.tx, mock.Anything).Return(models.CaseEvent{}, nil).Once()
			}
			uc := s.makeCaseUsecase()
			err := uc.applyCaseEntityChanges(s.ctx, s.tx, s.org, "case", "", []models.CaseEntityRef{ref}, tt.add)
			s.NoError(err)
			s.feature.AssertNotCalled(s.T(), "GetOrganizationFeatureAccess", mock.Anything, mock.Anything, mock.Anything)
			s.queue.AssertNotCalled(s.T(), "EnqueueTriggerScoreComputation", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

type CaseEntityMutationSuite struct {
	suite.Suite
	ctx         context.Context
	orgId       uuid.UUID
	inboxId     uuid.UUID
	repo        *mocks.CaseRepository
	decisions   *mocks.CaseEntityDecisionsRepository
	security    *mocks.EnforceSecurity
	inboxRepo   *mocks.InboxRepository
	feature     *mocks.FeatureAccessReader
	queue       *mocks.TaskQueueRepository
	dataModel   *mocks.DataModelRepository
	scoringRepo *mocks.ScoringRepository
	execFactory *mocks.ExecutorFactory
	exec        *mocks.Executor
	tx          *mocks.Transaction
	factory     *mocks.TransactionFactory
}

func (s *CaseEntityMutationSuite) SetupTest() {
	s.ctx = context.Background()
	s.orgId, s.inboxId = uuid.New(), uuid.New()
	s.repo = new(mocks.CaseRepository)
	s.decisions = new(mocks.CaseEntityDecisionsRepository)
	s.security = new(mocks.EnforceSecurity)
	s.inboxRepo = new(mocks.InboxRepository)
	s.feature = new(mocks.FeatureAccessReader)
	s.queue = new(mocks.TaskQueueRepository)
	s.dataModel = new(mocks.DataModelRepository)
	s.scoringRepo = new(mocks.ScoringRepository)
	s.execFactory = new(mocks.ExecutorFactory)
	s.exec = new(mocks.Executor)
	s.tx = new(mocks.Transaction)
	s.factory = &mocks.TransactionFactory{TxMock: s.tx}
}

func (s *CaseEntityMutationSuite) TearDownTest() {
	for _, m := range []*mock.Mock{
		&s.repo.Mock,
		&s.decisions.Mock,
		&s.security.Mock,
		&s.inboxRepo.Mock,
		&s.feature.Mock,
		&s.queue.Mock,
		&s.dataModel.Mock,
		&s.scoringRepo.Mock,
		&s.execFactory.Mock,
		&s.exec.Mock,
		&s.tx.Mock,
		&s.factory.Mock,
	} {
		m.AssertExpectations(s.T())
	}
}

func (s *CaseEntityMutationSuite) SetupSubTest()    { s.SetupTest() }
func (s *CaseEntityMutationSuite) TearDownSubTest() { s.TearDownTest() }

func (s *CaseEntityMutationSuite) makeUsecase() CaseUseCase {
	rulesets := scoring.NewScoringRulesetsUsecase(nil, s.execFactory, nil,
		feature_access.FeatureAccessReader{}, nil, s.scoringRepo, nil, nil, nil)
	return CaseUseCase{
		repository:         s.repo,
		decisionRepository: s.decisions,
		enforceSecurity:    s.security,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity: s.security,
			InboxRepository: s.inboxRepo,
			Credentials:     models.Credentials{Role: models.API_CLIENT},
		},
		transactionFactory:  s.factory,
		featureAccessReader: s.feature,
		taskQueueRepository: s.queue,
		scoringScoreUsecase: scoring.NewScoringScoresUsecase(
			nil, nil, nil, nil, rulesets, nil, s.dataModel, repositories.OffloadedReadWriter{}, nil, s.queue, ast_eval.EvaluateAstExpression{}, nil,
		),
	}
}

func (s *CaseEntityMutationSuite) TestScoresRefreshWhenOutcomeProvided() {
	for _, tt := range []struct {
		name    string
		outcome models.CaseOutcome
	}{
		{name: "changed outcome", outcome: models.CaseFalsePositive},
		{name: "unchanged outcome with a name change", outcome: models.CaseConfirmedRisk},
	} {
		s.Run(tt.name, func() {
			c := models.Case{Id: uuid.NewString(), OrganizationId: s.orgId, InboxId: s.inboxId,
				Status: models.CaseClosed, Outcome: models.CaseConfirmedRisk}
			ref := models.CaseEntityRef{TableName: "customers", ObjectId: "customer-1"}
			s.factory.On("Transaction", s.ctx, mock.Anything).Return(nil).Once()
			s.repo.On("GetCaseById", s.ctx, s.tx, c.Id).Return(c, nil).Once()
			s.inboxRepo.On("ListInboxes", s.ctx, s.tx, s.orgId, []uuid.UUID(nil), false).Return([]models.Inbox{{Id: s.inboxId}}, nil).Once()
			s.security.On("ReadInbox", models.Inbox{Id: s.inboxId}).Return(nil).Once()
			s.security.On("ReadOrUpdateCase", c.GetMetadata(), []uuid.UUID{s.inboxId}).Return(nil).Once()
			s.feature.On("GetOrganizationFeatureAccess", s.ctx, s.orgId, (*models.UserId)(nil)).Return(models.OrganizationFeatureAccess{UserScoring: models.Allowed}, nil).Once()
			expectedErr := fmt.Errorf("stop before case side effects")
			s.repo.On("UpdateCase", s.ctx, s.tx, mock.Anything).Return(expectedErr).Once()
			s.decisions.On("DecisionsByCaseId", s.ctx, s.tx, s.orgId, c.Id).Return([]models.Decision{}, nil).Once()
			s.dataModel.On("GetDataModel", s.ctx, s.tx, s.orgId, false, false).Return(models.DataModel{}, nil).Once()
			s.repo.On("ListCaseManualEntities", s.ctx, s.tx, s.orgId, c.Id).Return([]models.CaseManualEntity{{CaseEntityRef: ref}}, nil).Once()
			s.execFactory.On("NewExecutor").Return(s.exec).Once()
			s.scoringRepo.On("GetScoringRuleset", s.ctx, s.exec, s.orgId, ref.TableName, models.ScoreRulesetCommitted, 0).
				Return(models.ScoringRuleset{}, nil).Once()
			// An enqueue failure is logged and does not prevent the outcome update.
			s.queue.On("EnqueueTriggerScoreComputation", s.ctx, s.tx, models.ScoringRecordRef{
				OrgId: s.orgId, RecordType: ref.TableName, RecordId: ref.ObjectId,
			}).Return(fmt.Errorf("queue unavailable")).Once()
			uc := s.makeUsecase()
			_, err := uc.UpdateCase(s.ctx, "", models.UpdateCaseAttributes{Id: c.Id, Name: "Updated name", Outcome: tt.outcome})
			s.ErrorIs(err, expectedErr)
		})
	}
}

func (s *CaseEntityMutationSuite) TestClosedCaseAdditionAndRemoval() {
	tests := []struct {
		name   string
		mutate func(*CaseUseCase, context.Context, string, string, []models.CaseEntityRef) (models.Case, error)
	}{
		{name: "addition", mutate: (*CaseUseCase).AddCaseEntities},
		{name: "removal", mutate: (*CaseUseCase).RemoveCaseEntities},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			c := models.Case{Id: uuid.NewString(), OrganizationId: s.orgId, InboxId: s.inboxId, Status: models.CaseClosed}
			refs := []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-123"}}
			s.factory.On("Transaction", s.ctx, mock.Anything).Return(nil).Once()
			s.repo.On("GetCaseByIdForUpdate", s.ctx, s.tx, c.Id).Return(c.GetMetadata(), nil).Once()
			s.inboxRepo.On("ListInboxes", s.ctx, s.tx, s.orgId, []uuid.UUID(nil), false).Return([]models.Inbox{{Id: s.inboxId}}, nil).Once()
			s.security.On("ReadInbox", models.Inbox{Id: s.inboxId}).Return(nil).Once()
			s.security.On("ReadOrUpdateCase", c.GetMetadata(), []uuid.UUID{s.inboxId}).Return(nil).Once()
			uc := s.makeUsecase()

			_, err := tt.mutate(&uc, s.ctx, "actor", c.Id, refs)

			s.ErrorIs(err, models.BadParameterError)
			s.repo.AssertNotCalled(s.T(), "InsertCaseManualEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			s.repo.AssertNotCalled(s.T(), "DeleteCaseManualEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			s.repo.AssertNotCalled(s.T(), "CreateCaseEvent", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
func TestCaseEntityMutationSuite(t *testing.T) { suite.Run(t, new(CaseEntityMutationSuite)) }
