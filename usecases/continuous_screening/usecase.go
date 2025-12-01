package continuous_screening

import (
	"context"
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/usecases/security"
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
		orgId string,
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
	GetDataModelTable(ctx context.Context, exec repositories.Executor, tableId string) (models.TableMetadata, error)
	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID string,
		fetchEnumValues bool,
		useCache bool,
	) (models.DataModel, error)

	// Continuous screenings
	InsertContinuousScreening(
		ctx context.Context,
		exec repositories.Executor,
		screening models.ScreeningWithMatches,
		config models.ContinuousScreeningConfig,
		objectType string,
		objectId string,
		objectInternalId uuid.UUID,
		triggerType models.ContinuousScreeningTriggerType,
	) (models.ContinuousScreeningWithMatches, error)
	ListContinuousScreeningsForOrg(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		paginationAndSorting models.PaginationAndSorting,
	) ([]models.ContinuousScreeningWithMatches, error)
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

	// Cases:
	GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error)
	CreateCaseEvent(
		ctx context.Context,
		exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes,
	) error

	// Whitelist:
	AddScreeningMatchWhitelist(
		ctx context.Context,
		exec repositories.Executor,
		orgId string,
		counterpartyId string,
		entityId string,
		reviewerId *models.UserId,
	) error
	SearchScreeningMatchWhitelist(ctx context.Context, exec repositories.Executor,
		orgId string, counterpartyId, entityId *string,
	) ([]models.ScreeningWhitelist, error)
}

type inboxReader interface {
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error)
	ListInboxes(ctx context.Context, exec repositories.Executor, orgId string, withCaseCount bool) ([]models.Inbox, error)
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

type ContinuousScreeningClientDbRepository interface {
	CreateInternalContinuousScreeningTable(ctx context.Context, exec repositories.Executor) error
	CreateInternalContinuousScreeningAuditTable(ctx context.Context, exec repositories.Executor) error
	InsertContinuousScreeningObject(
		ctx context.Context,
		exec repositories.Executor,
		objectType string,
		objectId string,
		configStableId uuid.UUID,
	) error
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
		organizationId string,
		objectType string,
		objectBody json.RawMessage,
		parserOpts ...payload_parser.ParserOpt,
	) (int, error)
}

type ContinuousScreeningScreeningProvider interface {
	Search(
		ctx context.Context,
		query models.OpenSanctionsQuery,
	) (models.ScreeningRawSearchResponseWithMatches, error)
	GetAlgorithms(ctx context.Context) (models.OpenSanctionAlgorithms, error)
}

type ContinuousScreeningUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	enforceSecurity              security.EnforceSecurityContinuousScreening
	enforceSecurityCase          security.EnforceSecurityCase
	enforceSecurityScreening     security.EnforceSecurityScreening
	repository                   ContinuousScreeningUsecaseRepository
	clientDbRepository           ContinuousScreeningClientDbRepository
	organizationSchemaRepository repositories.OrganizationSchemaRepository
	ingestedDataReader           ContinuousScreeningIngestedDataReader
	ingestionUsecase             ContinuousScreeningIngestionUsecase
	screeningProvider            ContinuousScreeningScreeningProvider
	caseEditor                   caseEditor
	inboxReader                  inboxReader
}

func NewContinuousScreeningUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	enforceSecurity security.EnforceSecurityContinuousScreening,
	enforceSecurityCase security.EnforceSecurityCase,
	enforceSecurityScreening security.EnforceSecurityScreening,
	repository ContinuousScreeningUsecaseRepository,
	clientDbRepository ContinuousScreeningClientDbRepository,
	organizationSchemaRepository repositories.OrganizationSchemaRepository,
	ingestedDataReader ContinuousScreeningIngestedDataReader,
	ingestionUsecase ContinuousScreeningIngestionUsecase,
	screeningProvider ContinuousScreeningScreeningProvider,
	caseEditor caseEditor,
	inboxReader inboxReader,
) *ContinuousScreeningUsecase {
	return &ContinuousScreeningUsecase{
		executorFactory:              executorFactory,
		transactionFactory:           transactionFactory,
		enforceSecurity:              enforceSecurity,
		enforceSecurityCase:          enforceSecurityCase,
		enforceSecurityScreening:     enforceSecurityScreening,
		repository:                   repository,
		clientDbRepository:           clientDbRepository,
		organizationSchemaRepository: organizationSchemaRepository,
		ingestedDataReader:           ingestedDataReader,
		ingestionUsecase:             ingestionUsecase,
		screeningProvider:            screeningProvider,
		caseEditor:                   caseEditor,
		inboxReader:                  inboxReader,
	}
}
