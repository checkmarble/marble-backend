package usecases

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
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
	dm := usecase{dataModelRepository: s.model, enforceSecurity: s.security, executorFactory: s.factory, clientDbIndexEditor: s.indexes}
	s.reader = caseEntityReader{dataModelUsecase: dm, clientDbRepository: s.objects, executorFactory: s.factory}
}
func (s *CaseEntityReaderSuite) TearDownTest() {
	s.model.AssertExpectations(s.T())
	s.security.AssertExpectations(s.T())
	s.factory.AssertExpectations(s.T())
	s.objects.AssertExpectations(s.T())
	s.indexes.AssertExpectations(s.T())
}
func (s *CaseEntityReaderSuite) expectModel(dm models.DataModel) {
	s.factory.On("NewExecutor").Return(s.exec).Once()
	s.model.On("GetDataModel", s.ctx, s.exec, s.org, false, true).Return(dm, nil).Once()
}
func (s *CaseEntityReaderSuite) SetupSubTest()    { s.SetupTest() }
func (s *CaseEntityReaderSuite) TearDownSubTest() { s.TearDownTest() }

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
			repo := new(mocks.CaseRepository)
			feature := new(mocks.FeatureAccessReader)
			queue := new(mocks.TaskQueueRepository)
			tx := new(mocks.Transaction)
			var link *models.CaseManualEntity
			if tt.changed {
				link = &models.CaseManualEntity{Id: uuid.NewString(), CaseEntityRef: ref}
			}
			method := "DeleteCaseManualEntity"
			if tt.add {
				method = "InsertCaseManualEntity"
			}
			repo.On(method, s.ctx, tx, s.org, "case", ref).Return(link, nil).Once()
			if tt.changed {
				if tt.add {
					table := models.Table{Name: ref.TableName, SemanticType: models.SemanticTypePerson}
					s.expectModel(models.DataModel{Tables: map[string]models.Table{table.Name: table}})
					s.factory.On("NewClientDbExecutor", s.ctx, s.org).Return(s.exec, nil).Once()
					s.objects.On("QueryIngestedObjectByUniqueField", s.ctx, s.exec, table, ref.ObjectId, "object_id", []string(nil)).Return([]models.DataModelObject{{}}, nil).Once()
				}
				repo.On("CreateCaseEvent", s.ctx, tx, mock.Anything).Return(models.CaseEvent{}, nil).Once()
			}
			uc := CaseUseCase{repository: repo, caseEntityReader: s.reader, featureAccessReader: feature, taskQueueRepository: queue}
			err := uc.applyCaseEntityChanges(s.ctx, tx, s.org, "case", "", []models.CaseEntityRef{ref}, tt.add)
			s.NoError(err)
			repo.AssertExpectations(s.T())
			feature.AssertExpectations(s.T())
			queue.AssertExpectations(s.T())
			feature.AssertNotCalled(s.T(), "GetOrganizationFeatureAccess", mock.Anything, mock.Anything, mock.Anything)
			queue.AssertNotCalled(s.T(), "EnqueueTriggerScoreComputation", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

type CaseEntityMutationSuite struct{ suite.Suite }

type caseEntityDecisionsRepositoryMock struct {
	repositories.DecisionRepository
	mock.Mock
}

func (m *caseEntityDecisionsRepositoryMock) DecisionsByCaseId(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, caseId string) ([]models.Decision, error) {
	args := m.Called(ctx, exec, orgId, caseId)
	return args.Get(0).([]models.Decision), args.Error(1)
}

func (s *CaseEntityMutationSuite) TestScoresRefreshOnlyWhenOutcomeChanges() {
	for _, tt := range []struct {
		name    string
		outcome models.CaseOutcome
	}{
		{name: "changed outcome", outcome: models.CaseFalsePositive},
		{name: "unchanged outcome with a name change", outcome: models.CaseConfirmedRisk},
	} {
		s.Run(tt.name, func() {
			ctx := context.Background()
			orgId, inboxId := uuid.New(), uuid.New()
			c := models.Case{Id: uuid.NewString(), OrganizationId: orgId, InboxId: inboxId,
				Status: models.CaseClosed, Outcome: models.CaseConfirmedRisk}
			ref := models.CaseEntityRef{TableName: "customers", ObjectId: "customer-1"}
			repo := new(mocks.CaseRepository)
			decisions := new(caseEntityDecisionsRepositoryMock)
			security := new(mocks.EnforceSecurity)
			inboxRepo := new(mocks.InboxRepository)
			feature := new(mocks.FeatureAccessReader)
			queue := new(mocks.TaskQueueRepository)
			dataModel := new(mocks.DataModelRepository)
			tx := new(mocks.Transaction)
			factory := &mocks.TransactionFactory{TxMock: tx}
			factory.On("Transaction", ctx, mock.Anything).Return(nil).Once()
			repo.On("GetCaseById", ctx, tx, c.Id).Return(c, nil).Once()
			inboxRepo.On("ListInboxes", ctx, tx, orgId, []uuid.UUID(nil), false).Return([]models.Inbox{{Id: inboxId}}, nil).Once()
			security.On("ReadInbox", models.Inbox{Id: inboxId}).Return(nil).Once()
			security.On("ReadOrUpdateCase", c.GetMetadata(), []uuid.UUID{inboxId}).Return(nil).Once()
			feature.On("GetOrganizationFeatureAccess", ctx, orgId, (*models.UserId)(nil)).Return(models.OrganizationFeatureAccess{UserScoring: models.Allowed}, nil).Once()
			expectedErr := fmt.Errorf("stop before case side effects")
			if tt.outcome != c.Outcome {
				decisions.On("DecisionsByCaseId", ctx, tx, orgId, c.Id).Return([]models.Decision{}, nil).Once()
				dataModel.On("GetDataModel", ctx, tx, orgId, false, false).Return(models.DataModel{}, nil).Once()
				repo.On("ListCaseManualEntities", ctx, tx, orgId, c.Id).Return([]models.CaseManualEntity{{CaseEntityRef: ref}}, nil).Once()
				// An enqueue failure aborts before writing the changed outcome.
				queue.On("EnqueueTriggerScoreComputation", ctx, tx, models.ScoringRecordRef{
					OrgId: orgId, RecordType: ref.TableName, RecordId: ref.ObjectId,
				}).Return(expectedErr).Once()
			} else {
				repo.On("UpdateCase", ctx, tx, mock.Anything).Return(expectedErr).Once()
			}
			uc := CaseUseCase{
				repository: repo, decisionRepository: decisions, enforceSecurity: security,
				inboxReader: inboxes.InboxReader{EnforceSecurity: security, InboxRepository: inboxRepo,
					Credentials: models.Credentials{Role: models.API_CLIENT}},
				transactionFactory: factory, featureAccessReader: feature, taskQueueRepository: queue,
				scoringScoreUsecase: scoring.NewScoringScoresUsecase(nil, nil, nil, nil,
					scoring.ScoringRulesetsUsecase{}, nil, dataModel, repositories.OffloadedReadWriter{}, nil, nil,
					ast_eval.EvaluateAstExpression{}, nil),
			}
			_, err := uc.UpdateCase(ctx, "", models.UpdateCaseAttributes{Id: c.Id, Name: "Updated name", Outcome: tt.outcome})
			s.ErrorIs(err, expectedErr)
			if tt.outcome == c.Outcome {
				decisions.AssertNotCalled(s.T(), "DecisionsByCaseId", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				repo.AssertNotCalled(s.T(), "ListCaseManualEntities", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				queue.AssertNotCalled(s.T(), "EnqueueTriggerScoreComputation", mock.Anything, mock.Anything, mock.Anything)
			} else {
				repo.AssertNotCalled(s.T(), "UpdateCase", mock.Anything, mock.Anything, mock.Anything)
			}
			for _, m := range []*mock.Mock{&repo.Mock, &decisions.Mock, &security.Mock, &inboxRepo.Mock,
				&feature.Mock, &queue.Mock, &dataModel.Mock, &factory.Mock} {
				m.AssertExpectations(s.T())
			}
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
			ctx := context.Background()
			org := uuid.New()
			inbox := uuid.New()
			c := models.Case{Id: uuid.NewString(), OrganizationId: org, InboxId: inbox, Status: models.CaseClosed}
			refs := []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-123"}}
			repo := new(mocks.CaseRepository)
			security := new(mocks.EnforceSecurity)
			inboxRepo := new(mocks.InboxRepository)
			tx := new(mocks.Transaction)
			factory := &mocks.TransactionFactory{TxMock: tx}
			factory.On("Transaction", ctx, mock.Anything).Return(nil).Once()
			repo.On("GetCaseByIdForUpdate", ctx, tx, c.Id).Return(c.GetMetadata(), nil).Once()
			inboxRepo.On("ListInboxes", ctx, tx, org, []uuid.UUID(nil), false).Return([]models.Inbox{{Id: inbox}}, nil).Once()
			security.On("ReadInbox", models.Inbox{Id: inbox}).Return(nil).Once()
			security.On("ReadOrUpdateCase", c.GetMetadata(), []uuid.UUID{inbox}).Return(nil).Once()
			uc := CaseUseCase{
				repository:      repo,
				enforceSecurity: security,
				inboxReader: inboxes.InboxReader{
					EnforceSecurity: security,
					InboxRepository: inboxRepo,
					Credentials:     models.Credentials{Role: models.API_CLIENT},
				},
				transactionFactory: factory,
			}

			_, err := tt.mutate(&uc, ctx, "actor", c.Id, refs)

			s.ErrorIs(err, models.BadParameterError)
			repo.AssertNotCalled(s.T(), "InsertCaseManualEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			repo.AssertNotCalled(s.T(), "DeleteCaseManualEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			repo.AssertNotCalled(s.T(), "CreateCaseEvent", mock.Anything, mock.Anything, mock.Anything)
			repo.AssertExpectations(s.T())
			security.AssertExpectations(s.T())
			inboxRepo.AssertExpectations(s.T())
			factory.AssertExpectations(s.T())
		})
	}
}
func TestCaseEntityMutationSuite(t *testing.T) { suite.Run(t, new(CaseEntityMutationSuite)) }
