package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCaseEntityScoreComputations(t *testing.T) {
	ctx := context.Background()
	org := pure_utils.NewId()
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
	partner := models.CaseEntityRef{TableName: "partners", ObjectId: "c-123"}
	queueErr := errors.New("queue unavailable")
	featureErr := errors.New("feature access unavailable")
	for _, tt := range []struct {
		name       string
		refs       []models.CaseEntityRef
		access     models.FeatureAccess
		featureErr error
		queueErr   error
		wantErr    error
	}{
		{name: "empty links"},
		{name: "scoring disabled", refs: []models.CaseEntityRef{ref}},
		{name: "deduplicate by table and ID", refs: []models.CaseEntityRef{ref, ref, partner}, access: models.Allowed},
		{name: "queue failure", refs: []models.CaseEntityRef{ref, partner}, access: models.Allowed, queueErr: queueErr, wantErr: queueErr},
		{name: "feature failure", refs: []models.CaseEntityRef{ref}, featureErr: featureErr, wantErr: featureErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tx := new(mocks.Transaction)
			features := new(mocks.FeatureAccessReader)
			queue := new(mocks.TaskQueueRepository)
			if len(tt.refs) > 0 {
				features.On("GetOrganizationFeatureAccess", ctx, org, (*models.UserId)(nil)).
					Return(models.OrganizationFeatureAccess{UserScoring: tt.access}, tt.featureErr).Once()
			}
			if tt.access.IsAllowed() && tt.featureErr == nil {
				queue.On("EnqueueManyTriggerScoreComputation", ctx, tx, []models.ScoringRecordRef{
					{OrgId: org, RecordType: ref.TableName, RecordId: ref.ObjectId},
					{OrgId: org, RecordType: partner.TableName, RecordId: partner.ObjectId},
				}).Return(tt.queueErr).Once()
			}
			uc := CaseUseCase{featureAccessReader: features, taskQueueRepository: queue}
			err := uc.enqueueCaseEntityScoreComputations(ctx, tx, org, tt.refs)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			features.AssertExpectations(t)
			queue.AssertExpectations(t)
		})
	}
}

type manualCaseDecisionsRepository struct {
	repositories.DecisionRepository
}

func (manualCaseDecisionsRepository) DecisionsByCaseId(context.Context, repositories.Executor, uuid.UUID, string) ([]models.Decision, error) {
	return nil, nil
}

func TestCaseOutcomeChangeRefreshesManualEntities(t *testing.T) {
	for _, outcome := range []models.CaseOutcome{models.CaseConfirmedRisk, models.CaseFalsePositive, models.CaseOutcomeUnset} {
		t.Run(string(outcome), func(t *testing.T) {
			ctx := context.Background()
			org, inbox := pure_utils.NewId(), pure_utils.NewId()
			c := models.Case{Id: pure_utils.NewId().String(), OrganizationId: org, InboxId: inbox,
				Status: models.CaseInvestigating, Outcome: models.CaseConfirmedRisk}
			if outcome == models.CaseConfirmedRisk {
				c.Outcome = models.CaseOutcomeUnset
			}
			ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
			tx := new(mocks.Transaction)
			factory := &mocks.TransactionFactory{TxMock: tx}
			repo := new(mocks.CaseRepository)
			security := new(mocks.EnforceSecurity)
			inboxRepo := new(mocks.InboxRepository)
			features := new(mocks.FeatureAccessReader)
			queue := new(mocks.TaskQueueRepository)
			factory.On("Transaction", ctx, mock.Anything).Return(nil).Once()
			repo.On("GetCaseById", ctx, tx, c.Id).Return(c, nil).Once()
			inboxRepo.On("ListInboxes", ctx, tx, org, []uuid.UUID(nil), false).Return([]models.Inbox{{Id: inbox}}, nil).Once()
			security.On("ReadInbox", models.Inbox{Id: inbox}).Return(nil).Once()
			security.On("ReadOrUpdateCase", c.GetMetadata(), []uuid.UUID{inbox}).Return(nil).Once()
			features.On("GetOrganizationFeatureAccess", ctx, org, (*models.UserId)(nil)).
				Return(models.OrganizationFeatureAccess{UserScoring: models.Allowed}, nil).Twice()
			update := models.UpdateCaseAttributes{Id: c.Id, Outcome: outcome}
			updated := repo.On("UpdateCase", ctx, tx, update).Return(nil).Once()
			repo.On("ListCaseManualEntities", ctx, tx, org, c.Id).
				Return([]models.CaseManualEntity{{CaseEntityRef: ref}}, nil).NotBefore(updated).Once()
			queueErr := errors.New("queue unavailable")
			queue.On("EnqueueManyTriggerScoreComputation", ctx, tx, []models.ScoringRecordRef{
				{OrgId: org, RecordType: ref.TableName, RecordId: ref.ObjectId},
			}).Return(queueErr).Once()
			uc := CaseUseCase{
				repository: repo, transactionFactory: factory, enforceSecurity: security,
				featureAccessReader: features, taskQueueRepository: queue,
				decisionRepository: manualCaseDecisionsRepository{},
				inboxReader: inboxes.InboxReader{EnforceSecurity: security, InboxRepository: inboxRepo,
					Credentials: models.Credentials{Role: models.API_CLIENT}},
			}
			// A queue failure must reach the transaction factory so the outcome and
			// its corresponding refresh request are rolled back together.
			_, err := uc.UpdateCase(ctx, "actor", update)
			require.ErrorIs(t, err, queueErr)
			repo.AssertExpectations(t)
			factory.AssertExpectations(t)
			security.AssertExpectations(t)
			inboxRepo.AssertExpectations(t)
			features.AssertExpectations(t)
			queue.AssertExpectations(t)
		})
	}
}

func TestRemovedCaseEntityRefreshesScore(t *testing.T) {
	for _, changed := range []bool{true, false} {
		t.Run(map[bool]string{true: "effective removal", false: "already absent"}[changed], func(t *testing.T) {
			ctx := context.Background()
			org := pure_utils.NewId()
			caseID := pure_utils.NewId().String()
			ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
			tx := new(mocks.Transaction)
			repo := new(mocks.CaseRepository)
			features := new(mocks.FeatureAccessReader)
			queue := new(mocks.TaskQueueRepository)
			var link *models.CaseManualEntity
			if changed {
				link = &models.CaseManualEntity{Id: pure_utils.NewId().String(), CaseEntityRef: ref}
				repo.On("CreateCaseEvent", ctx, tx, mock.MatchedBy(func(event models.CreateCaseEventAttributes) bool {
					return event.EventType == models.CaseEntityRemoved && event.PreviousValue != nil
				})).Return(models.CaseEvent{}, nil).Once()
				features.On("GetOrganizationFeatureAccess", ctx, org, (*models.UserId)(nil)).
					Return(models.OrganizationFeatureAccess{UserScoring: models.Allowed}, nil).Once()
				queue.On("EnqueueManyTriggerScoreComputation", ctx, tx, []models.ScoringRecordRef{
					{OrgId: org, RecordType: ref.TableName, RecordId: ref.ObjectId},
				}).Return(nil).Once()
			}
			repo.On("DeleteCaseManualEntity", ctx, tx, org, caseID, ref).Return(link, nil).Once()
			uc := CaseUseCase{repository: repo, featureAccessReader: features, taskQueueRepository: queue}
			require.NoError(t, uc.applyCaseEntityChanges(ctx, tx, org, caseID, "", []models.CaseEntityRef{ref}, false))
			repo.AssertExpectations(t)
			features.AssertExpectations(t)
			queue.AssertExpectations(t)
		})
	}
}

func (s *CaseEntityReaderSuite) TestAddedCaseEntityRefreshesScore() {
	table := models.Table{Name: "customers", SemanticType: models.SemanticTypePerson}
	ref := models.CaseEntityRef{TableName: table.Name, ObjectId: "c-123"}
	s.expectModel(models.DataModel{Tables: map[string]models.Table{table.Name: table}})
	s.factory.On("NewClientDbExecutor", s.ctx, s.org).Return(s.exec, nil).Once()
	s.objects.On("QueryIngestedObjectByUniqueField", s.ctx, s.exec, table, ref.ObjectId, "object_id", []string(nil)).
		Return([]models.DataModelObject{{Data: map[string]any{"object_id": ref.ObjectId}}}, nil).Once()
	caseID := pure_utils.NewId().String()
	link := &models.CaseManualEntity{Id: pure_utils.NewId().String(), CaseEntityRef: ref}
	tx := new(mocks.Transaction)
	repo := new(mocks.CaseRepository)
	features := new(mocks.FeatureAccessReader)
	queue := new(mocks.TaskQueueRepository)
	repo.On("InsertCaseManualEntity", s.ctx, tx, s.org, caseID, ref).Return(link, nil).Once()
	repo.On("CreateCaseEvent", s.ctx, tx, mock.MatchedBy(func(event models.CreateCaseEventAttributes) bool {
		return event.EventType == models.CaseEntityAdded && event.NewValue != nil
	})).Return(models.CaseEvent{}, nil).Once()
	features.On("GetOrganizationFeatureAccess", s.ctx, s.org, (*models.UserId)(nil)).
		Return(models.OrganizationFeatureAccess{UserScoring: models.Allowed}, nil).Once()
	queue.On("EnqueueManyTriggerScoreComputation", s.ctx, tx, []models.ScoringRecordRef{
		{OrgId: s.org, RecordType: ref.TableName, RecordId: ref.ObjectId},
	}).Return(nil).Once()
	uc := CaseUseCase{repository: repo, caseEntityReader: s.reader, featureAccessReader: features, taskQueueRepository: queue}
	s.NoError(uc.applyCaseEntityChanges(s.ctx, tx, s.org, caseID, "", []models.CaseEntityRef{ref}, true))
	repo.AssertExpectations(s.T())
	features.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}
