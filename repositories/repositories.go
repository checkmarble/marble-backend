package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

type options struct {
	metabase                      Metabase
	transfercheckEnrichmentBucket string
	convoyClientProvider          ConvoyClientProvider
	convoyRateLimit               int
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

type Repositories struct {
	ExecutorGetter                    ExecutorGetter
	ConvoyRepository                  ConvoyRepository
	UserRepository                    UserRepository
	OrganizationRepository            OrganizationRepository
	IngestionRepository               IngestionRepository
	DataModelRepository               DataModelRepository
	IngestedDataReadRepository        IngestedDataReadRepository
	DecisionRepository                DecisionRepository
	MarbleDbRepository                MarbleDbRepository
	ClientDbRepository                ClientDbRepository
	ScenarioPublicationRepository     ScenarioPublicationRepository
	OrganizationSchemaRepository      OrganizationSchemaRepository
	AwsS3Repository                   AwsS3Repository
	BlobRepository                    BlobRepository
	CustomListRepository              CustomListRepository
	UploadLogRepository               UploadLogRepository
	MarbleAnalyticsRepository         MarbleAnalyticsRepository
	TransferCheckEnrichmentRepository *TransferCheckEnrichmentRepository
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

	executorGetter := NewExecutorGetter(marbleConnectionPool)

	blobRepository := NewBlobRepository(googleApplicationCredentials)

	return Repositories{
		ExecutorGetter:                executorGetter,
		ConvoyRepository:              NewConvoyRepository(options.convoyClientProvider, options.convoyRateLimit),
		UserRepository:                &UserRepositoryPostgresql{},
		OrganizationRepository:        &OrganizationRepositoryPostgresql{},
		IngestionRepository:           &IngestionRepositoryImpl{},
		DataModelRepository:           &DataModelRepositoryPostgresql{},
		IngestedDataReadRepository:    &IngestedDataReadRepositoryImpl{},
		DecisionRepository:            &DecisionRepositoryImpl{},
		MarbleDbRepository:            MarbleDbRepository{},
		ClientDbRepository:            ClientDbRepository{},
		ScenarioPublicationRepository: &ScenarioPublicationRepositoryPostgresql{},
		OrganizationSchemaRepository:  &OrganizationSchemaRepositoryPostgresql{},
		CustomListRepository:          &CustomListRepositoryPostgresql{},
		UploadLogRepository:           &UploadLogRepositoryImpl{},
		AwsS3Repository:               AwsS3Repository{s3Client: NewS3Client()},
		BlobRepository:                blobRepository,
		MarbleAnalyticsRepository: MarbleAnalyticsRepository{
			metabase: options.metabase,
		},
		TransferCheckEnrichmentRepository: NewTransferCheckEnrichmentRepository(
			blobRepository,
			options.transfercheckEnrichmentBucket,
		),
	}
}
