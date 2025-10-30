package screening_monitoring

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

type ScreeningMonitoringUsecaseRepository interface {
	GetScreeningMonitoringConfig(
		ctx context.Context,
		exec repositories.Executor,
		Id uuid.UUID,
	) (
		models.ScreeningMonitoringConfig, error)
	GetScreeningMonitoringConfigsByOrgId(
		ctx context.Context,
		exec repositories.Executor,
		orgId string,
	) (
		[]models.ScreeningMonitoringConfig, error)
	CreateScreeningMonitoringConfig(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateScreeningMonitoringConfig,
	) (models.ScreeningMonitoringConfig, error)
	UpdateScreeningMonitoringConfig(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
		input models.UpdateScreeningMonitoringConfig,
	) (models.ScreeningMonitoringConfig, error)
	GetDataModelTable(ctx context.Context, exec repositories.Executor, tableId string) (models.TableMetadata, error)
	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID string,
		fetchEnumValues bool,
		useCache bool,
	) (models.DataModel, error)
}

type ScreeningMonitoringClientDbRepository interface {
	CreateInternalScreeningMonitoringTable(ctx context.Context, exec repositories.Executor, tableName string) error
	CreateInternalScreeningMonitoringIndex(ctx context.Context, exec repositories.Executor, tableName string) error
	InsertScreeningMonitoringObject(
		ctx context.Context,
		exec repositories.Executor,
		tableName string,
		objectId string,
		configId uuid.UUID,
	) error
}

type ScreeningMonitoringIngestedDataReader interface {
	QueryIngestedObject(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		objectId string,
	) ([]models.DataModelObject, error)
}

type ScreeningMonitoringIngestionUsecase interface {
	IngestObject(
		ctx context.Context,
		organizationId string,
		objectType string,
		objectBody json.RawMessage,
		parserOpts ...payload_parser.ParserOpt,
	) (int, error)
}

type ScreeningMonitoringUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	enforceSecurity              security.EnforceSecurityScreeningMonitoring
	repository                   ScreeningMonitoringUsecaseRepository
	clientDbRepository           ScreeningMonitoringClientDbRepository
	organizationSchemaRepository repositories.OrganizationSchemaRepository
	ingestedDataReader           ScreeningMonitoringIngestedDataReader
	ingestionUsecase             ScreeningMonitoringIngestionUsecase
}

func NewScreeningMonitoringUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	enforceSecurity security.EnforceSecurityScreeningMonitoring,
	screeningMonitoringRepository ScreeningMonitoringUsecaseRepository,
	clientDbRepository ScreeningMonitoringClientDbRepository,
	organizationSchemaRepository repositories.OrganizationSchemaRepository,
	ingestedDataReader ScreeningMonitoringIngestedDataReader,
	ingestionUsecase ScreeningMonitoringIngestionUsecase,
) ScreeningMonitoringUsecase {
	return ScreeningMonitoringUsecase{
		executorFactory:              executorFactory,
		transactionFactory:           transactionFactory,
		enforceSecurity:              enforceSecurity,
		repository:                   screeningMonitoringRepository,
		clientDbRepository:           clientDbRepository,
		organizationSchemaRepository: organizationSchemaRepository,
		ingestedDataReader:           ingestedDataReader,
		ingestionUsecase:             ingestionUsecase,
	}
}
