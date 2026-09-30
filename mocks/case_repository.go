package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CaseRepository struct {
	mock.Mock
}

func (r *CaseRepository) ListOrganizationCases(ctx context.Context, tx repositories.Transaction,
	filters models.CaseFilters, pagination models.PaginationAndSorting,
) ([]models.Case, error) {
	args := r.Called(ctx, tx, filters, pagination)
	return args.Get(0).([]models.Case), args.Error(1)
}

func (r *CaseRepository) GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).(models.Case), args.Error(1)
}

func (r *CaseRepository) GetCaseMetadataById(ctx context.Context, exec repositories.Executor, caseId string) (models.CaseMetadata, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).(models.CaseMetadata), args.Error(1)
}

func (r *CaseRepository) CreateCase(ctx context.Context, exec repositories.Executor,
	createCaseAttributes models.CreateCaseAttributes, newCaseId string,
) error {
	args := r.Called(ctx, exec, createCaseAttributes, newCaseId)
	return args.Error(0)
}

func (r *CaseRepository) UpdateCase(ctx context.Context, exec repositories.Executor, updateCaseAttributes models.UpdateCaseAttributes) error {
	args := r.Called(ctx, exec, updateCaseAttributes)
	return args.Error(0)
}

func (r *CaseRepository) SnoozeCase(ctx context.Context, exec repositories.Executor, snoozeRequest models.CaseSnoozeRequest) error {
	args := r.Called(ctx, exec, snoozeRequest)
	return args.Error(0)
}

func (r *CaseRepository) UnsnoozeCase(ctx context.Context, exec repositories.Executor, caseId string) error {
	args := r.Called(ctx, exec, caseId)
	return args.Error(0)
}

func (r *CaseRepository) GetCaseReferents(ctx context.Context, exec repositories.Executor, caseIds []string) ([]models.CaseReferents, error) {
	args := r.Called(ctx, exec, caseIds)
	return args.Get(0).([]models.CaseReferents), args.Error(1)
}

func (r *CaseRepository) DecisionPivotValuesByCase(ctx context.Context, exec repositories.Executor, caseId string) ([]models.PivotDataWithCount, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).([]models.PivotDataWithCount), args.Error(1)
}

func (r *CaseRepository) CreateCaseEvent(ctx context.Context, exec repositories.Executor,
	createCaseEventAttributes models.CreateCaseEventAttributes,
) (models.CaseEvent, error) {
	args := r.Called(ctx, exec, createCaseEventAttributes)
	return args.Get(0).(models.CaseEvent), args.Error(1)
}

func (r *CaseRepository) BatchCreateCaseEvents(ctx context.Context, exec repositories.Executor,
	createCaseEventAttributes []models.CreateCaseEventAttributes,
) ([]models.CaseEvent, error) {
	args := r.Called(ctx, exec, createCaseEventAttributes)
	return args.Get(0).([]models.CaseEvent), args.Error(1)
}

func (r *CaseRepository) ListCaseEvents(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseEvent, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).([]models.CaseEvent), args.Error(1)
}

func (r *CaseRepository) ListCaseEventsOfTypes(ctx context.Context, exec repositories.Executor,
	caseId string, types []models.CaseEventType, paging models.PaginationAndSorting,
) ([]models.CaseEvent, error) {
	args := r.Called(ctx, exec, caseId, types, paging)
	return args.Get(0).([]models.CaseEvent), args.Error(1)
}

func (r *CaseRepository) GetCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) (*models.CaseContributor, error) {
	args := r.Called(ctx, exec, caseId, userId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CaseContributor), args.Error(1)
}

func (r *CaseRepository) CreateCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) error {
	args := r.Called(ctx, exec, caseId, userId)
	return args.Error(0)
}

func (r *CaseRepository) GetTagById(ctx context.Context, exec repositories.Executor, tagId string) (models.Tag, error) {
	args := r.Called(ctx, exec, tagId)
	return args.Get(0).(models.Tag), args.Error(1)
}

func (r *CaseRepository) CreateCaseTag(ctx context.Context, exec repositories.Executor, caseId, tagId string) error {
	args := r.Called(ctx, exec, caseId, tagId)
	return args.Error(0)
}

func (r *CaseRepository) ListCaseTagsByCaseId(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseTag, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).([]models.CaseTag), args.Error(1)
}

func (r *CaseRepository) SoftDeleteCaseTag(ctx context.Context, exec repositories.Executor, tagId string) error {
	args := r.Called(ctx, exec, tagId)
	return args.Error(0)
}

func (r *CaseRepository) CreateDbCaseFile(ctx context.Context, exec repositories.Executor,
	createCaseFileInput models.CreateDbCaseFileInput,
) (models.CaseFile, error) {
	args := r.Called(ctx, exec, createCaseFileInput)
	return args.Get(0).(models.CaseFile), args.Error(1)
}

func (r *CaseRepository) GetCaseFileById(ctx context.Context, exec repositories.Executor, caseFileId string) (models.CaseFile, error) {
	args := r.Called(ctx, exec, caseFileId)
	return args.Get(0).(models.CaseFile), args.Error(1)
}

func (r *CaseRepository) GetCasesFileByCaseId(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseFile, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).([]models.CaseFile), args.Error(1)
}

func (r *CaseRepository) AssignCase(ctx context.Context, exec repositories.Executor, id string, userId *models.UserId) error {
	args := r.Called(ctx, exec, id, userId)
	return args.Error(0)
}

func (r *CaseRepository) UnassignCase(ctx context.Context, exec repositories.Executor, id string) error {
	args := r.Called(ctx, exec, id)
	return args.Error(0)
}

func (r *CaseRepository) BoostCase(ctx context.Context, exec repositories.Executor, id string, reason models.BoostReason) error {
	args := r.Called(ctx, exec, id, reason)
	return args.Error(0)
}

func (r *CaseRepository) UnboostCase(ctx context.Context, exec repositories.Executor, id string) error {
	args := r.Called(ctx, exec, id)
	return args.Error(0)
}

func (r *CaseRepository) EscalateCase(ctx context.Context, exec repositories.Executor, id, inboxId string) error {
	args := r.Called(ctx, exec, id, inboxId)
	return args.Error(0)
}

func (r *CaseRepository) GetCasesWithPivotValue(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, pivotValue string) ([]models.Case, error) {
	args := r.Called(ctx, exec, orgId, pivotValue)
	return args.Get(0).([]models.Case), args.Error(1)
}

func (r *CaseRepository) GetContinuousScreeningCasesWithObjectAttr(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID, objectType, objectId string,
) ([]models.Case, error) {
	args := r.Called(ctx, exec, orgId, objectType, objectId)
	return args.Get(0).([]models.Case), args.Error(1)
}

func (r *CaseRepository) GetNextCase(ctx context.Context, exec repositories.Executor, c models.Case) (string, error) {
	args := r.Called(ctx, exec, c)
	return args.String(0), args.Error(1)
}

func (r *CaseRepository) UserById(ctx context.Context, exec repositories.Executor, userId string) (models.User, error) {
	args := r.Called(ctx, exec, userId)
	return args.Get(0).(models.User), args.Error(1)
}

func (r *CaseRepository) GetMassCasesByIds(ctx context.Context, exec repositories.Executor, caseIds []uuid.UUID) ([]models.Case, error) {
	args := r.Called(ctx, exec, caseIds)
	return args.Get(0).([]models.Case), args.Error(1)
}

func (r *CaseRepository) CaseMassChangeStatus(ctx context.Context, tx repositories.Transaction,
	caseIds []uuid.UUID, status models.CaseStatus,
) ([]uuid.UUID, error) {
	args := r.Called(ctx, tx, caseIds, status)
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

func (r *CaseRepository) CaseMassAssign(ctx context.Context, tx repositories.Transaction,
	caseIds []uuid.UUID, assigneeId uuid.UUID,
) ([]uuid.UUID, error) {
	args := r.Called(ctx, tx, caseIds, assigneeId)
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

func (r *CaseRepository) CaseMassMoveToInbox(ctx context.Context, tx repositories.Transaction,
	caseIds []uuid.UUID, inboxId uuid.UUID,
) ([]uuid.UUID, error) {
	args := r.Called(ctx, tx, caseIds, inboxId)
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

func (r *CaseRepository) ListContinuousScreeningsWithMatchesByCaseId(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID, caseId string,
) ([]models.ContinuousScreeningWithMatches, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).([]models.ContinuousScreeningWithMatches), args.Error(1)
}

func (r *CaseRepository) ListContinuousScreeningsByIds(ctx context.Context,
	exec repositories.Executor, ids []uuid.UUID,
) ([]models.ContinuousScreening, error) {
	args := r.Called(ctx, exec, ids)
	return args.Get(0).([]models.ContinuousScreening), args.Error(1)
}

func (r *CaseRepository) UpdateContinuousScreeningsCaseId(ctx context.Context,
	exec repositories.Executor, ids []uuid.UUID, caseId string,
) error {
	args := r.Called(ctx, exec, ids, caseId)
	return args.Error(0)
}

func (r *CaseRepository) GetContinuousScreeningConfig(ctx context.Context,
	exec repositories.Executor, id uuid.UUID,
) (models.ContinuousScreeningConfig, error) {
	args := r.Called(ctx, exec, id)
	return args.Get(0).(models.ContinuousScreeningConfig), args.Error(1)
}

func (r *CaseRepository) ListContinuousScreeningMatchCommentsByMatchIds(
	ctx context.Context,
	exec repositories.Executor,
	ids []uuid.UUID,
) ([]models.ScreeningMatchComment, error) {
	args := r.Called(ctx, exec, ids)
	return args.Get(0).([]models.ScreeningMatchComment), args.Error(1)
}

func (r *CaseRepository) GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error) {
	args := r.Called(ctx, exec, inboxId)
	return args.Get(0).(models.Inbox), args.Error(1)
}

func (r *CaseRepository) ListCaseManualEntities(arg0 context.Context, arg1 repositories.Executor, arg2 uuid.UUID, caseId string) ([]models.CaseManualEntity, error) {
	args := r.Called(arg0, arg1, arg2, caseId)
	var value0 []models.CaseManualEntity
	if args.Get(0) != nil {
		value0 = args.Get(0).([]models.CaseManualEntity)
	}
	return value0, args.Error(1)
}

func (r *CaseRepository) InsertCaseManualEntity(arg0 context.Context, arg1 repositories.Executor, arg2 uuid.UUID, caseId string, arg4 models.CaseEntityRef) (*models.CaseManualEntity, error) {
	args := r.Called(arg0, arg1, arg2, caseId, arg4)
	var value0 *models.CaseManualEntity
	if args.Get(0) != nil {
		value0 = args.Get(0).(*models.CaseManualEntity)
	}
	return value0, args.Error(1)
}

func (r *CaseRepository) DeleteCaseManualEntity(arg0 context.Context, arg1 repositories.Executor, arg2 uuid.UUID, caseId string, arg4 models.CaseEntityRef) (*models.CaseManualEntity, error) {
	args := r.Called(arg0, arg1, arg2, caseId, arg4)
	var value0 *models.CaseManualEntity
	if args.Get(0) != nil {
		value0 = args.Get(0).(*models.CaseManualEntity)
	}
	return value0, args.Error(1)
}

func (r *CaseRepository) GetCaseByIdForUpdate(ctx context.Context, exec repositories.Executor, caseId string) (models.CaseMetadata, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).(models.CaseMetadata), args.Error(1)
}

func (r *CaseRepository) DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error) {
	args := r.Called(ctx, exec, decisionIds)
	var value0 []models.Decision
	if args.Get(0) != nil {
		value0 = args.Get(0).([]models.Decision)
	}
	return value0, args.Error(1)
}

func (r *CaseRepository) ListCaseCommentEvents(ctx context.Context, exec repositories.Executor, caseId string, paging models.PaginationAndSorting) ([]models.CaseCommentEvent, error) {
	args := r.Called(ctx, exec, caseId, paging)
	var value0 []models.CaseCommentEvent
	if args.Get(0) != nil {
		value0 = args.Get(0).([]models.CaseCommentEvent)
	}
	return value0, args.Error(1)
}

func (r *CaseRepository) GetContinuousScreeningCasesByEntityIdInMatches(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, entityId string) ([]models.Case, error) {
	args := r.Called(ctx, exec, orgId, entityId)
	var value0 []models.Case
	if args.Get(0) != nil {
		value0 = args.Get(0).([]models.Case)
	}
	return value0, args.Error(1)
}

func (r *CaseRepository) GetCasesRelatedToObject(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, objectType string, objectId string) ([]models.Case, error) {
	args := r.Called(ctx, exec, orgId, objectType, objectId)
	var value0 []models.Case
	if args.Get(0) != nil {
		value0 = args.Get(0).([]models.Case)
	}
	return value0, args.Error(1)
}
