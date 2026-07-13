package continuous_screening

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ContinuousScreeningUsecaseRepository interface {
	// Configs
	GetContinuousScreeningConfig(
		ctx context.Context,
		exec repositories.Executor,
		Id uuid.UUID,
	) (models.ContinuousScreeningConfig, error)
	GetContinuousScreeningConfigsByOrgId(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		provider models.ScreeningProvider,
	) ([]models.ContinuousScreeningConfig, error)
	GetContinuousScreeningConfigByStableId(
		ctx context.Context,
		exec repositories.Executor,
		stableId uuid.UUID,
	) (models.ContinuousScreeningConfig, error)
	CreateContinuousScreeningConfig(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningConfig,
	) (models.ContinuousScreeningConfig, error)
	UpdateContinuousScreeningConfig(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		input models.UpdateContinuousScreeningConfig,
	) (models.ContinuousScreeningConfig, error)

	// Data model
	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID uuid.UUID,
		fetchEnumValues bool,
		useCache bool,
	) (models.DataModel, error)

	// Continuous screenings
	InsertContinuousScreening(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreening,
	) (models.ContinuousScreeningWithMatches, error)
	ListContinuousScreeningsForOrg(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		paginationAndSorting models.PaginationAndSorting,
	) ([]models.ContinuousScreeningWithMatches, error)
	ListContinuousScreeningDatasetUpdates(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		provider models.ScreeningProvider,
		pagination models.PaginationAndSorting,
	) ([]models.ContinuousScreeningDatasetUpdateEnriched, error)
	ListContinuousScreeningUpdateJobs(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		pagination models.PaginationAndSorting,
	) ([]models.ContinuousScreeningUpdateJobSummary, error)
	ListContinuousScreeningClientDataIndexing(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		provider models.ScreeningProvider,
		indexVersion *string,
		pagination models.PaginationAndSorting,
	) (models.ContinuousScreeningClientDataIndexing, error)
	GetContinuousScreeningWithMatchesById(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
	) (models.ContinuousScreeningWithMatches, error)
	UpdateContinuousScreeningStatus(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		newStatus models.ScreeningStatus,
	) (models.ContinuousScreening, error)
	UpdateContinuousScreening(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		input models.UpdateContinuousScreeningInput,
	) (models.ContinuousScreening, error)
	UpdateContinuousScreeningEntityEnrichedPayload(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		enrichedPayload []byte,
	) error
	UpdateContinuousScreeningMatchEnrichedPayload(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		enrichedPayload []byte,
	) error

	// Continuous screening matches
	GetContinuousScreeningMatch(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
	) (models.ContinuousScreeningMatch, error)
	UpdateContinuousScreeningMatchStatus(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		status models.ScreeningMatchStatus,
		reviewerId *uuid.UUID,
	) (models.ContinuousScreeningMatch, error)
	UpdateContinuousScreeningMatchStatusByBatch(
		ctx context.Context,
		exec repositories.Executor,
		ids []uuid.UUID,
		status models.ScreeningMatchStatus,
		reviewerId *uuid.UUID,
	) ([]models.ContinuousScreeningMatch, error)
	InsertContinuousScreeningMatches(
		ctx context.Context,
		exec repositories.Executor,
		screeningId uuid.UUID,
		matches []models.ContinuousScreeningMatch,
	) ([]models.ContinuousScreeningMatch, error)
	AddContinuousScreeningMatchComment(
		ctx context.Context,
		exec repositories.Executor,
		comment models.ScreeningMatchComment,
	) (models.ScreeningMatchComment, error)
	ListContinuousScreeningMatchCommentsByMatchIds(
		ctx context.Context,
		exec repositories.Executor,
		ids []uuid.UUID,
	) ([]models.ScreeningMatchComment, error)

	// Cases:
	GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error)
	CreateCaseEvent(
		ctx context.Context,
		exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes,
	) (models.CaseEvent, error)
	BatchCreateCaseEvents(
		ctx context.Context,
		exec repositories.Executor,
		createCaseEventAttributes []models.CreateCaseEventAttributes,
	) ([]models.CaseEvent, error)

	// Whitelist:
	AddScreeningMatchWhitelist(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		counterpartyId string,
		entityId string,
		reviewerId *models.UserId,
	) error
	SearchScreeningMatchWhitelist(ctx context.Context, exec repositories.Executor,
		orgId uuid.UUID, counterpartyId, entityId *string,
	) ([]models.ScreeningWhitelist, error)
	SearchScreeningMatchWhitelistByIds(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		counterpartyIds, entityIds []string,
	) ([]models.ScreeningWhitelist, error)

	// Data model
	UpdateDataModelTable(
		ctx context.Context,
		exec repositories.Executor,
		tableID string,
		description *string,
		ftmEntity pure_utils.Null[models.FollowTheMoneyEntity],
		alias pure_utils.Null[string],
		semanticType pure_utils.Null[models.SemanticType],
		captionField pure_utils.Null[string],
		primaryOrderingField pure_utils.Null[string],
		metadata *json.RawMessage,
	) error
	UpdateDataModelField(
		ctx context.Context,
		exec repositories.Executor,
		fieldId string,
		input models.UpdateFieldInput,
	) error

	// Dataset files
	CreateContinuousScreeningDeltaTrack(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningDeltaTrack,
	) error

	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) (models.Organization, error)
}

type continuousScreeningTaskQueueRepository interface {
	EnqueueContinuousScreeningMatchEnrichmentTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		continuousScreeningId uuid.UUID,
	) error
	EnqueueContinuousScreeningEnsureDeltaTrackTask(
		ctx context.Context,
		args models.ContinuousScreeningEnsureDeltaTrackArgs,
	) error
}

type inboxReader interface {
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error)
	ListInboxes(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, withCaseCount bool) ([]models.Inbox, error)
}

type inboxEditor interface {
	CreateInboxWithExecutor(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateInboxInput,
	) (models.Inbox, error)
}

type featureAccessReader interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		organizationId uuid.UUID,
		userId *models.UserId,
	) (models.OrganizationFeatureAccess, error)
}

type objectRiskTagWriter interface {
	AttachObjectRiskTags(
		ctx context.Context,
		tx repositories.Transaction,
		input models.ObjectRiskTagCreate,
	) error
}

type caseEditor interface {
	CreateCase(
		ctx context.Context,
		tx repositories.Transaction,
		userId string,
		createCaseAttributes models.CreateCaseAttributes,
		fromEndUser bool,
	) (models.Case, error)
	PerformCaseActionSideEffects(ctx context.Context, tx repositories.Transaction, caseModel models.Case) error
}

type webhookEventsUsecase interface {
	CreateWebhookEvent(
		ctx context.Context,
		tx repositories.Transaction,
		input models.WebhookEventCreate,
	) error
}

type ContinuousScreeningClientDbRepository interface {
	CreateInternalContinuousScreeningTable(ctx context.Context, exec repositories.Executor) error
	CreateInternalContinuousScreeningAuditTable(ctx context.Context, exec repositories.Executor) error
	InsertContinuousScreeningObject(
		ctx context.Context,
		exec repositories.Executor,
		objectType string,
		objectId string,
		configStableId uuid.UUID,
	) (uuid.UUID, error)
	InsertContinuousScreeningAudit(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningAudit,
	) error
	DeleteContinuousScreeningObject(
		ctx context.Context,
		exec repositories.Executor,
		input models.DeleteContinuousScreeningObject,
	) error
	ListMonitoredObjects(
		ctx context.Context,
		exec repositories.Executor,
		filters models.ListMonitoredObjectsFilters,
		pagination models.PaginationAndSorting,
	) ([]models.ContinuousScreeningMonitoredObject, error)
	ListMonitoredObjectsByObjectIds(
		ctx context.Context,
		exec repositories.Executor,
		objectType string,
		objectIds []string,
	) ([]models.ContinuousScreeningMonitoredObject, error)
}

type ContinuousScreeningIngestedDataReader interface {
	QueryIngestedObject(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		objectId string,
		metadataFields ...string,
	) ([]models.DataModelObject, error)
}

type ContinuousScreeningIngestionUsecase interface {
	IngestObject(
		ctx context.Context,
		organizationId uuid.UUID,
		objectType string,
		objectBody json.RawMessage,
		ingestionOptions models.IngestionOptions,
		parserOpts ...payload_parser.ParserOpt,
	) (int, error)
}

type ContinuousScreeningScreeningProvider interface {
	GetCatalog(ctx context.Context, provider models.ScreeningProvider) (models.OpenSanctionsCatalog, error)
	GetRawCatalog(ctx context.Context, provider models.ScreeningProvider) (models.OpenSanctionsRawCatalog, error)
	Search(
		ctx context.Context,
		providerName models.ScreeningProvider,
		query models.OpenSanctionsQuery,
	) (models.ScreeningRawSearchResponseWithMatches, error)
	GetAlgorithms(ctx context.Context) (models.OpenSanctionAlgorithms, error)
	EnrichMatch(ctx context.Context, providerName models.ScreeningProvider, match models.ScreeningMatch) ([]byte, error)
}

type ContinuousScreeningUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	enforceSecurity              security.EnforceSecurityContinuousScreening
	enforceSecurityCase          security.EnforceSecurityCase
	enforceSecurityScreening     security.EnforceSecurityScreening
	repository                   ContinuousScreeningUsecaseRepository
	taskQueueRepository          continuousScreeningTaskQueueRepository
	clientDbRepository           ContinuousScreeningClientDbRepository
	organizationSchemaRepository repositories.OrganizationSchemaRepository
	ingestedDataReader           ContinuousScreeningIngestedDataReader
	ingestionUsecase             ContinuousScreeningIngestionUsecase
	screeningProvider            ContinuousScreeningScreeningProvider
	caseEditor                   caseEditor
	inboxReader                  inboxReader
	inboxEditor                  inboxEditor
	featureAccessReader          featureAccessReader
	objectRiskTagWriter          objectRiskTagWriter
	webhookEventsUsecase         webhookEventsUsecase
	offloadedReader              repositories.OffloadedReadWriter
}

func NewContinuousScreeningUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	enforceSecurity security.EnforceSecurityContinuousScreening,
	enforceSecurityCase security.EnforceSecurityCase,
	enforceSecurityScreening security.EnforceSecurityScreening,
	repository ContinuousScreeningUsecaseRepository,
	taskQueueRepository continuousScreeningTaskQueueRepository,
	clientDbRepository ContinuousScreeningClientDbRepository,
	organizationSchemaRepository repositories.OrganizationSchemaRepository,
	ingestedDataReader ContinuousScreeningIngestedDataReader,
	ingestionUsecase ContinuousScreeningIngestionUsecase,
	screeningProvider ContinuousScreeningScreeningProvider,
	caseEditor caseEditor,
	inboxReader inboxReader,
	inboxEditor inboxEditor,
	featureAccessReader featureAccessReader,
	objectRiskTagWriter objectRiskTagWriter,
	webhookEventsUsecase webhookEventsUsecase,
	offloadedReader repositories.OffloadedReadWriter,
) *ContinuousScreeningUsecase {
	return &ContinuousScreeningUsecase{
		executorFactory:              executorFactory,
		transactionFactory:           transactionFactory,
		enforceSecurity:              enforceSecurity,
		enforceSecurityCase:          enforceSecurityCase,
		enforceSecurityScreening:     enforceSecurityScreening,
		repository:                   repository,
		taskQueueRepository:          taskQueueRepository,
		clientDbRepository:           clientDbRepository,
		organizationSchemaRepository: organizationSchemaRepository,
		ingestedDataReader:           ingestedDataReader,
		ingestionUsecase:             ingestionUsecase,
		screeningProvider:            screeningProvider,
		caseEditor:                   caseEditor,
		inboxReader:                  inboxReader,
		inboxEditor:                  inboxEditor,
		featureAccessReader:          featureAccessReader,
		objectRiskTagWriter:          objectRiskTagWriter,
		webhookEventsUsecase:         webhookEventsUsecase,
		offloadedReader:              offloadedReader,
	}
}

// hydrateContinuousScreenings loads offloaded entity and match payloads from blob storage (no-op
// when offloading is disabled) and then sorts each screening's matches by status, then descending
// score. The score lives in the (possibly offloaded) payload, so the sort runs in memory here
// rather than in SQL. The slice is mutated in place.
func (uc *ContinuousScreeningUsecase) hydrateContinuousScreenings(
	ctx context.Context, screenings []models.ContinuousScreeningWithMatches,
) error {
	for i := range screenings {
		if err := uc.offloadedReader.HydrateContinuousScreeningEntity(ctx, &screenings[i]); err != nil {
			return errors.Wrap(err, "failed to hydrate continuous screening entity")
		}
		if err := uc.offloadedReader.HydrateContinuousScreeningMatch(ctx, &screenings[i]); err != nil {
			return errors.Wrap(err, "failed to hydrate continuous screening match")
		}

		slices.SortStableFunc(screenings[i].Matches, func(a, b models.ContinuousScreeningMatch) int {
			if n := cmp.Compare(continuousScreeningMatchStatusRank(a.Status),
				continuousScreeningMatchStatusRank(b.Status)); n != 0 {
				return n
			}
			return cmp.Compare(b.GetScoreFromPayload(), a.GetScoreFromPayload())
		})
	}

	return nil
}

// hydrateContinuousScreening hydrates and sorts a single continuous screening in place. Unlike
// passing a value to hydrateContinuousScreenings, this reflects the hydrated entity payload back
// onto the caller's screening (a scalar field, not a shared slice).
func (uc *ContinuousScreeningUsecase) hydrateContinuousScreening(
	ctx context.Context, screening *models.ContinuousScreeningWithMatches,
) error {
	wrapped := []models.ContinuousScreeningWithMatches{*screening}
	if err := uc.hydrateContinuousScreenings(ctx, wrapped); err != nil {
		return err
	}
	*screening = wrapped[0]
	return nil
}

// continuousScreeningMatchStatusRank mirrors the status ordering previously expressed in SQL
// (array['confirmed_hit', 'pending', 'no_hit', 'skipped']).
func continuousScreeningMatchStatusRank(s models.ScreeningMatchStatus) int {
	switch s {
	case models.ScreeningMatchStatusConfirmedHit:
		return 0
	case models.ScreeningMatchStatusPending:
		return 1
	case models.ScreeningMatchStatusNoHit:
		return 2
	case models.ScreeningMatchStatusSkipped:
		return 3
	default:
		return 4
	}
}
