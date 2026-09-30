package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
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
	reader   IngestedDataReaderUsecase
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
	s.reader = IngestedDataReaderUsecase{repository: s.model, dataModelUsecase: dm, clientDbRepository: s.objects, executorFactory: s.factory}
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
func TestCaseEntityReaderSuite(t *testing.T) { suite.Run(t, new(CaseEntityReaderSuite)) }

type CaseEntityMutationSuite struct{ suite.Suite }

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
			uc := CaseUseCase{repository: repo, enforceSecurity: security, inboxReader: inboxes.InboxReader{EnforceSecurity: security, InboxRepository: inboxRepo, Credentials: models.Credentials{Role: models.API_CLIENT}}, transactionFactory: factory}

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
