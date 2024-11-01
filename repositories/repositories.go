package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"go.opentelemetry.io/otel/trace"
)

type options struct {
	metabase                      Metabase
	transfercheckEnrichmentBucket string
	clientDbConfig                map[string]infra.ClientDbConfig
	convoyClientProvider          ConvoyClientProvider
	convoyRateLimit               int
	riverClient                   *river.Client[pgx.Tx]
	tp                            trace.TracerProvider
}

type Option func(*options)

func getOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithMetabase(metabase Metabase) Option {
	return func(o *options) {
		o.metabase = metabase
	}
}

func WithTransferCheckEnrichmentBucket(bucket string) Option {
	return func(o *options) {
		o.transfercheckEnrichmentBucket = bucket
	}
}

func WithConvoyClientProvider(convoyResources ConvoyClientProvider, convoyRateLimit int) Option {
	return func(o *options) {
		o.convoyClientProvider = convoyResources
		o.convoyRateLimit = convoyRateLimit
	}
}

func WithRiverClient(client *river.Client[pgx.Tx]) Option {
	return func(o *options) {
		o.riverClient = client
	}
}

func WithClientDbConfig(clientDbConfig map[string]infra.ClientDbConfig) Option {
	return func(o *options) {
		o.clientDbConfig = clientDbConfig
	}
}

func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(o *options) {
		o.tp = tp
	}
}

type Repositories struct {
	ExecutorGetter                    ExecutorGetter
	ConvoyRepository                  ConvoyRepository
	UserRepository                    UserRepository
	OrganizationRepository            OrganizationRepository
	IngestionRepository               IngestionRepository
	DataModelRepository               DataModelRepository
	IngestedDataReadRepository        IngestedDataReadRepository
	MarbleDbRepository                MarbleDbRepository
	ClientDbRepository                ClientDbRepository
	ScenarioPublicationRepository     ScenarioPublicationRepository
	OrganizationSchemaRepository      OrganizationSchemaRepository
	BlobRepository                    BlobRepository
	CustomListRepository              CustomListRepository
	UploadLogRepository               UploadLogRepository
	MarbleAnalyticsRepository         MarbleAnalyticsRepository
	TransferCheckEnrichmentRepository *TransferCheckEnrichmentRepository
	TaskQueueRepository               TaskQueueRepository
}

func NewQueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}

func NewRepositories(
	marbleConnectionPool *pgxpool.Pool,
	googleApplicationCredentials string,
	opts ...Option,
) Repositories {
	options := getOptions(opts)

	executorGetter := NewExecutorGetter(marbleConnectionPool, options.clientDbConfig, options.tp)

	blobRepository := NewBlobRepository(googleApplicationCredentials)

	return Repositories{
		ExecutorGetter:                executorGetter,
		ConvoyRepository:              NewConvoyRepository(options.convoyClientProvider, options.convoyRateLimit),
		UserRepository:                &UserRepositoryPostgresql{},
		OrganizationRepository:        &OrganizationRepositoryPostgresql{},
		IngestionRepository:           &IngestionRepositoryImpl{},
		DataModelRepository:           &DataModelRepositoryPostgresql{},
		IngestedDataReadRepository:    &IngestedDataReadRepositoryImpl{},
		MarbleDbRepository:            MarbleDbRepository{},
		ClientDbRepository:            ClientDbRepository{},
		ScenarioPublicationRepository: &ScenarioPublicationRepositoryPostgresql{},
		OrganizationSchemaRepository:  &OrganizationSchemaRepositoryPostgresql{},
		CustomListRepository:          &CustomListRepositoryPostgresql{},
		UploadLogRepository:           &UploadLogRepositoryImpl{},
		BlobRepository:                blobRepository,
		MarbleAnalyticsRepository: MarbleAnalyticsRepository{
			metabase: options.metabase,
		},
		TransferCheckEnrichmentRepository: NewTransferCheckEnrichmentRepository(
			blobRepository,
			options.transfercheckEnrichmentBucket,
		),
		TaskQueueRepository: NewTaskQueueRepository(options.riverClient),
	}
}
