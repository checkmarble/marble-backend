package repositories

import (
	"fmt"
	"net/http"

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
	openSanctions                 infra.OpenSanctions
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

func WithOpenSanctions(openSanctionsConfig infra.OpenSanctions) Option {
	return func(o *options) {
		o.openSanctions = openSanctionsConfig
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
	IngestedDataReadRepository        IngestedDataReadRepository
	MarbleDbRepository                MarbleDbRepository
	ClientDbRepository                ClientDbRepository
	ScenarioPublicationRepository     ScenarioPublicationRepository
	OrganizationSchemaRepository      OrganizationSchemaRepository
	BlobRepository                    BlobRepository
	CustomListRepository              CustomListRepository
	UploadLogRepository               UploadLogRepository
	MarbleAnalyticsRepository         MarbleAnalyticsRepository
	OpenSanctionsRepository           OpenSanctionsRepository
	NameRecognitionRepository         NameRecognitionRepository
	TransferCheckEnrichmentRepository *TransferCheckEnrichmentRepository
	TaskQueueRepository               TaskQueueRepository
	ScenarioTestrunRepository         ScenarioTestRunRepository
}

func NewQueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}

func WithUnionAll(builder squirrel.SelectBuilder, unionAllQuery squirrel.SelectBuilder) (squirrel.SelectBuilder, error) {
	query, params, err := unionAllQuery.ToSql()
	if err != nil {
		return builder, fmt.Errorf("union all query failed: %w", err)
	}
	return builder.Suffix("UNION ALL "+query, params...), nil
}

func NewRepositories(
	marbleConnectionPool *pgxpool.Pool,
	gcpConfig infra.GcpConfig,
	opts ...Option,
) Repositories {
	options := getOptions(opts)

	executorGetter := NewExecutorGetter(marbleConnectionPool, options.clientDbConfig, options.tp)

	blobRepository := NewBlobRepository(gcpConfig)

	return Repositories{
		ExecutorGetter:                executorGetter,
		ConvoyRepository:              NewConvoyRepository(options.convoyClientProvider, options.convoyRateLimit),
		UserRepository:                &UserRepositoryPostgresql{},
		OrganizationRepository:        &OrganizationRepositoryPostgresql{},
		IngestionRepository:           &IngestionRepositoryImpl{},
		IngestedDataReadRepository:    &IngestedDataReadRepositoryImpl{},
		MarbleDbRepository:            MarbleDbRepository{},
		ScenarioTestrunRepository:     &MarbleDbRepository{},
		ClientDbRepository:            ClientDbRepository{},
		ScenarioPublicationRepository: &ScenarioPublicationRepositoryPostgresql{},
		OrganizationSchemaRepository:  &OrganizationSchemaRepositoryPostgresql{},
		CustomListRepository:          &CustomListRepositoryPostgresql{},
		UploadLogRepository:           &UploadLogRepositoryImpl{},
		BlobRepository:                blobRepository,
		MarbleAnalyticsRepository: MarbleAnalyticsRepository{
			metabase: options.metabase,
		},
		OpenSanctionsRepository: OpenSanctionsRepository{
			opensanctions: options.openSanctions,
		},
		NameRecognitionRepository: NameRecognitionRepository{
			NameRecognitionProvider: options.openSanctions.NameRecognition(),
			Client:                  http.DefaultClient,
		},
		TransferCheckEnrichmentRepository: NewTransferCheckEnrichmentRepository(
			blobRepository,
			options.transfercheckEnrichmentBucket,
		),
		TaskQueueRepository: NewTaskQueueRepository(options.riverClient),
	}
}
