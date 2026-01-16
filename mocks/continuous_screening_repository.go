package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type ContinuousScreeningRepository struct {
	mock.Mock
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
) (models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningConfigsByOrgId(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
) ([]models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, orgId)
	return args.Get(0).([]models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningConfigByStableId(
	ctx context.Context,
	exec repositories.Executor,
	stableId uuid.UUID,
) (models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, stableId)
	return args.Get(0).(models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetLastProcessedVersion(
	ctx context.Context,
	exec repositories.Executor,
	datasetName string,
) (models.ContinuousScreeningDatasetUpdate, error) {
	args := m.Called(ctx, exec, datasetName)
	return args.Get(0).(models.ContinuousScreeningDatasetUpdate), args.Error(1)
}

func (m *ContinuousScreeningRepository) CreateContinuousScreeningDatasetUpdate(
	ctx context.Context,
	exec repositories.Executor,
	input models.CreateContinuousScreeningDatasetUpdate,
) (models.ContinuousScreeningDatasetUpdate, error) {
	args := m.Called(ctx, exec, input)
	return args.Get(0).(models.ContinuousScreeningDatasetUpdate), args.Error(1)
}

func (m *ContinuousScreeningRepository) ListContinuousScreeningConfigs(
	ctx context.Context,
	exec repositories.Executor,
) ([]models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec)
	return args.Get(0).([]models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) CreateContinuousScreeningUpdateJob(
	ctx context.Context,
	exec repositories.Executor,
	input models.CreateContinuousScreeningUpdateJob,
) (models.ContinuousScreeningUpdateJob, error) {
	args := m.Called(ctx, exec, input)
	return args.Get(0).(models.ContinuousScreeningUpdateJob), args.Error(1)
}

func (m *ContinuousScreeningRepository) ListContinuousScreeningConfigByObjectType(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	objectType string,
) ([]models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, orgId, objectType)
	return args.Get(0).([]models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) CreateContinuousScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	input models.CreateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, input)
	return args.Get(0).(models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) UpdateContinuousScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
	input models.UpdateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, id, input)
	return args.Get(0).(models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetDataModelTable(
	ctx context.Context,
	exec repositories.Executor,
	tableId string,
) (models.TableMetadata, error) {
	args := m.Called(ctx, exec, tableId)
	return args.Get(0).(models.TableMetadata), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetDataModel(
	ctx context.Context,
	exec repositories.Executor,
	organizationID uuid.UUID,
	fetchEnumValues bool,
	useCache bool,
) (models.DataModel, error) {
	args := m.Called(ctx, exec, organizationID, fetchEnumValues, useCache)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (m *ContinuousScreeningRepository) InsertContinuousScreening(
	ctx context.Context,
	exec repositories.Executor,
	input models.CreateContinuousScreening,
) (models.ContinuousScreeningWithMatches, error) {
	args := m.Called(ctx, exec, input)
	return args.Get(0).(models.ContinuousScreeningWithMatches), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningByObjectId(
	ctx context.Context,
	exec repositories.Executor,
	objectId string,
	objectType string,
	orgId uuid.UUID,
	status *models.ScreeningStatus,
	inCase bool,
) (*models.ContinuousScreeningWithMatches, error) {
	args := m.Called(ctx, exec, objectId, objectType, orgId, status, inCase)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result := args.Get(0).(*models.ContinuousScreeningWithMatches)
	return result, args.Error(1)
}

func (m *ContinuousScreeningRepository) ListContinuousScreeningsForOrg(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ContinuousScreeningWithMatches, error) {
	args := m.Called(ctx, exec, orgId, paginationAndSorting)
	return args.Get(0).([]models.ContinuousScreeningWithMatches), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningWithMatchesById(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
) (models.ContinuousScreeningWithMatches, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.ContinuousScreeningWithMatches), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningMatch(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
) (models.ContinuousScreeningMatch, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.ContinuousScreeningMatch), args.Error(1)
}

func (m *ContinuousScreeningRepository) UpdateContinuousScreeningStatus(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
	newStatus models.ScreeningStatus,
) (models.ContinuousScreening, error) {
	args := m.Called(ctx, exec, id, newStatus)
	return args.Get(0).(models.ContinuousScreening), args.Error(1)
}

func (m *ContinuousScreeningRepository) UpdateContinuousScreeningMatchStatus(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
	status models.ScreeningMatchStatus,
	reviewerId *uuid.UUID,
) (models.ContinuousScreeningMatch, error) {
	args := m.Called(ctx, exec, id, status, reviewerId)
	return args.Get(0).(models.ContinuousScreeningMatch), args.Error(1)
}

func (m *ContinuousScreeningRepository) UpdateContinuousScreeningMatchStatusByBatch(
	ctx context.Context,
	exec repositories.Executor,
	ids []uuid.UUID,
	status models.ScreeningMatchStatus,
	reviewerId *uuid.UUID,
) ([]models.ContinuousScreeningMatch, error) {
	args := m.Called(ctx, exec, ids, status, reviewerId)
	return args.Get(0).([]models.ContinuousScreeningMatch), args.Error(1)
}

func (m *ContinuousScreeningRepository) SearchScreeningMatchWhitelist(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	counterpartyId, entityId *string,
) ([]models.ScreeningWhitelist, error) {
	args := m.Called(ctx, exec, orgId, counterpartyId, entityId)
	return args.Get(0).([]models.ScreeningWhitelist), args.Error(1)
}

func (m *ContinuousScreeningRepository) SearchScreeningMatchWhitelistByIds(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	counterpartyIds, entityIds []string,
) ([]models.ScreeningWhitelist, error) {
	args := m.Called(ctx, exec, orgId, counterpartyIds, entityIds)
	return args.Get(0).([]models.ScreeningWhitelist), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetInboxById(
	ctx context.Context,
	exec repositories.Executor,
	inboxId uuid.UUID,
) (models.Inbox, error) {
	args := m.Called(ctx, exec, inboxId)
	return args.Get(0).(models.Inbox), args.Error(1)
}

func (m *ContinuousScreeningRepository) ListInboxes(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	withCaseCount bool,
) ([]models.Inbox, error) {
	args := m.Called(ctx, exec, orgId, withCaseCount)
	return args.Get(0).([]models.Inbox), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetCaseById(
	ctx context.Context,
	exec repositories.Executor,
	caseId string,
) (models.Case, error) {
	args := m.Called(ctx, exec, caseId)
	return args.Get(0).(models.Case), args.Error(1)
}

func (m *ContinuousScreeningRepository) CreateCaseEvent(
	ctx context.Context,
	exec repositories.Executor,
	createCaseEventAttributes models.CreateCaseEventAttributes,
) error {
	args := m.Called(ctx, exec, createCaseEventAttributes)
	return args.Error(0)
}

func (m *ContinuousScreeningRepository) BatchCreateCaseEvents(
	ctx context.Context,
	exec repositories.Executor,
	createCaseEventAttributes []models.CreateCaseEventAttributes,
) error {
	args := m.Called(ctx, exec, createCaseEventAttributes)
	return args.Error(0)
}

func (m *ContinuousScreeningRepository) AddScreeningMatchWhitelist(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	counterpartyId string,
	entityId string,
	reviewerId *models.UserId,
) error {
	args := m.Called(ctx, exec, orgId, counterpartyId, entityId, reviewerId)
	return args.Error(0)
}

func (m *ContinuousScreeningRepository) InsertContinuousScreeningMatches(
	ctx context.Context,
	exec repositories.Executor,
	screeningId uuid.UUID,
	matches []models.ContinuousScreeningMatch,
) ([]models.ContinuousScreeningMatch, error) {
	args := m.Called(ctx, exec, screeningId, matches)
	return args.Get(0).([]models.ContinuousScreeningMatch), args.Error(1)
}

func (m *ContinuousScreeningRepository) UpdateContinuousScreening(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
	input models.UpdateContinuousScreeningInput,
) (models.ContinuousScreening, error) {
	args := m.Called(ctx, exec, id, input)
	return args.Get(0).(models.ContinuousScreening), args.Error(1)
}

func (m *ContinuousScreeningRepository) UpdateDataModelTable(
	ctx context.Context,
	exec repositories.Executor,
	tableID string,
	description *string,
	ftmEntity pure_utils.Null[models.FollowTheMoneyEntity],
) error {
	args := m.Called(ctx, exec, tableID, description, ftmEntity)
	return args.Error(0)
}

func (m *ContinuousScreeningRepository) UpdateDataModelField(
	ctx context.Context,
	exec repositories.Executor,
	fieldId string,
	input models.UpdateFieldInput,
) error {
	args := m.Called(ctx, exec, fieldId, input)
	return args.Error(0)
}

func (m *ContinuousScreeningRepository) CreateContinuousScreeningDeltaTrack(
	ctx context.Context,
	exec repositories.Executor,
	input models.CreateContinuousScreeningDeltaTrack,
) error {
	args := m.Called(ctx, exec, input)
	return args.Error(0)
}

func (m *ContinuousScreeningRepository) ListContinuousScreeningLatestFullFiles(
	ctx context.Context,
	exec repositories.Executor,
) ([]models.ContinuousScreeningDatasetFile, error) {
	args := m.Called(ctx, exec)
	return args.Get(0).([]models.ContinuousScreeningDatasetFile), args.Error(1)
}

func (m *ContinuousScreeningRepository) ListContinuousScreeningLatestDeltaFiles(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	limit uint64,
) ([]models.ContinuousScreeningDatasetFile, error) {
	args := m.Called(ctx, exec, orgId, limit)
	return args.Get(0).([]models.ContinuousScreeningDatasetFile), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetOrganizationById(
	ctx context.Context,
	exec repositories.Executor,
	organizationId uuid.UUID,
) (models.Organization, error) {
	args := m.Called(ctx, exec, organizationId)
	return args.Get(0).(models.Organization), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningDatasetFileById(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
) (models.ContinuousScreeningDatasetFile, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.ContinuousScreeningDatasetFile), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningLatestDatasetFileByOrgId(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	fileType models.ContinuousScreeningDatasetFileType,
) (*models.ContinuousScreeningDatasetFile, error) {
	args := m.Called(ctx, exec, orgId, fileType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContinuousScreeningDatasetFile), args.Error(1)
}

func (m *ContinuousScreeningRepository) UpdateContinuousScreeningEntityEnrichedPayload(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
	enrichedPayload []byte,
) error {
	args := m.Called(ctx, exec, id, enrichedPayload)
	return args.Error(0)
}

func (m *ContinuousScreeningRepository) UpdateContinuousScreeningMatchEnrichedPayload(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
	enrichedPayload []byte,
) error {
	args := m.Called(ctx, exec, id, enrichedPayload)
	return args.Error(0)
}

type ContinuousScreeningClientDbRepository struct {
	mock.Mock
}

func (m *ContinuousScreeningClientDbRepository) CreateInternalContinuousScreeningTable(
	ctx context.Context,
	exec repositories.Executor,
) error {
	args := m.Called(ctx, exec)
	return args.Error(0)
}

func (m *ContinuousScreeningClientDbRepository) CreateInternalContinuousScreeningAuditTable(
	ctx context.Context,
	exec repositories.Executor,
) error {
	args := m.Called(ctx, exec)
	return args.Error(0)
}

func (m *ContinuousScreeningClientDbRepository) InsertContinuousScreeningObject(
	ctx context.Context,
	exec repositories.Executor,
	tableName string,
	objectId string,
	configStableId uuid.UUID,
) error {
	args := m.Called(ctx, exec, tableName, objectId, configStableId)
	return args.Error(0)
}

func (m *ContinuousScreeningClientDbRepository) InsertContinuousScreeningAudit(
	ctx context.Context,
	exec repositories.Executor,
	audit models.CreateContinuousScreeningAudit,
) error {
	args := m.Called(ctx, exec, audit)
	return args.Error(0)
}

func (m *ContinuousScreeningClientDbRepository) DeleteContinuousScreeningObject(
	ctx context.Context,
	exec repositories.Executor,
	input models.DeleteContinuousScreeningObject,
) error {
	args := m.Called(ctx, exec, input)
	return args.Error(0)
}

func (m *ContinuousScreeningClientDbRepository) ListMonitoredObjectsByObjectIds(
	ctx context.Context,
	exec repositories.Executor,
	objectType string,
	objectIds []string,
) ([]models.ContinuousScreeningMonitoredObject, error) {
	args := m.Called(ctx, exec, objectType, objectIds)
	return args.Get(0).([]models.ContinuousScreeningMonitoredObject), args.Error(1)
}

func (m *ContinuousScreeningClientDbRepository) GetMonitoredObject(
	ctx context.Context,
	clientExec repositories.Executor,
	monitoringId uuid.UUID,
) (models.ContinuousScreeningMonitoredObject, error) {
	args := m.Called(ctx, clientExec, monitoringId)
	return args.Get(0).(models.ContinuousScreeningMonitoredObject), args.Error(1)
}

func (m *ContinuousScreeningClientDbRepository) ListMonitoredObjects(
	ctx context.Context,
	exec repositories.Executor,
	filters models.ListMonitoredObjectsFilters,
	pagination models.PaginationAndSorting,
) ([]models.ContinuousScreeningMonitoredObject, error) {
	args := m.Called(ctx, exec, filters, pagination)
	return args.Get(0).([]models.ContinuousScreeningMonitoredObject), args.Error(1)
}

type ContinuousScreeningScreeningProvider struct {
	mock.Mock
}

func (m *ContinuousScreeningScreeningProvider) Search(
	ctx context.Context,
	query models.OpenSanctionsQuery,
) (models.ScreeningRawSearchResponseWithMatches, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return models.ScreeningRawSearchResponseWithMatches{}, args.Error(1)
	}
	return args.Get(0).(models.ScreeningRawSearchResponseWithMatches), args.Error(1)
}

func (m *ContinuousScreeningScreeningProvider) GetAlgorithms(ctx context.Context) (models.OpenSanctionAlgorithms, error) {
	args := m.Called(ctx)
	return args.Get(0).(models.OpenSanctionAlgorithms), args.Error(1)
}

func (m *ContinuousScreeningScreeningProvider) EnrichMatch(ctx context.Context, match models.ScreeningMatch) ([]byte, error) {
	args := m.Called(ctx, match)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}
