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
	GetDataModelTable(ctx context.Context, exec repositories.Executor, tableId string) (models.TableMetadata, error)
	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID string,
		fetchEnumValues bool,
		useCache bool,
	) (models.DataModel, error)
	InsertContinuousScreening(
		ctx context.Context,
		exec repositories.Executor,
		screening models.ScreeningWithMatches,
		orgId uuid.UUID,
		configId uuid.UUID,
		configStableId uuid.UUID,
		objectType string,
		objectId string,
		objectInternalId uuid.UUID,
	) (models.ContinuousScreeningWithMatches, error)
	ListContinuousScreeningsForOrg(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		paginationAndSorting models.PaginationAndSorting,
	) ([]models.ContinuousScreeningWithMatches, error)

	// Inboxes:
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error)
}

type caseEditor interface {
	CreateCase(
		ctx context.Context,
		tx repositories.Transaction,
		userId string,
		createCaseAttributes models.CreateCaseAttributes,
		fromEndUser bool,
	) (models.Case, error)
}

type ContinuousScreeningClientDbRepository interface {
	CreateInternalContinuousScreeningTable(ctx context.Context, exec repositories.Executor, tableName string) error
	InsertContinuousScreeningObject(
		ctx context.Context,
		exec repositories.Executor,
		tableName string,
		objectId string,
		configStableId uuid.UUID,
	) error
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
	repository                   ContinuousScreeningUsecaseRepository
	clientDbRepository           ContinuousScreeningClientDbRepository
	organizationSchemaRepository repositories.OrganizationSchemaRepository
	ingestedDataReader           ContinuousScreeningIngestedDataReader
	ingestionUsecase             ContinuousScreeningIngestionUsecase
	screeningProvider            ContinuousScreeningScreeningProvider
	caseEditor                   caseEditor
}

func NewContinuousScreeningUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	enforceSecurity security.EnforceSecurityContinuousScreening,
	repository ContinuousScreeningUsecaseRepository,
	clientDbRepository ContinuousScreeningClientDbRepository,
	organizationSchemaRepository repositories.OrganizationSchemaRepository,
	ingestedDataReader ContinuousScreeningIngestedDataReader,
	ingestionUsecase ContinuousScreeningIngestionUsecase,
	screeningProvider ContinuousScreeningScreeningProvider,
	caseEditor caseEditor,
) ContinuousScreeningUsecase {
	return ContinuousScreeningUsecase{
		executorFactory:              executorFactory,
		transactionFactory:           transactionFactory,
		enforceSecurity:              enforceSecurity,
		repository:                   repository,
		clientDbRepository:           clientDbRepository,
		organizationSchemaRepository: organizationSchemaRepository,
		ingestedDataReader:           ingestedDataReader,
		ingestionUsecase:             ingestionUsecase,
		screeningProvider:            screeningProvider,
		caseEditor:                   caseEditor,
	}
}
