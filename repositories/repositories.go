package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

type options struct {
	metabase                      Metabase
	transfercheckEnrichmentBucket string
	fakeGcsRepository             bool
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

func WithFakeGcsRepository(b bool) Option {
	return func(o *options) {
		o.fakeGcsRepository = b
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
	GcsRepository                     GcsRepository
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
	opts ...Option,
) Repositories {
	options := getOptions(opts)

	executorGetter := NewExecutorGetter(marbleConnectionPool)

	var gcsRepository GcsRepository
	if options.fakeGcsRepository {
		gcsRepository = &GcsRepositoryFake{}
	} else {
		gcsRepository = &GcsRepositoryImpl{}
	}

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
		GcsRepository:                 gcsRepository,
		MarbleAnalyticsRepository: MarbleAnalyticsRepository{
			metabase: options.metabase,
		},
		TransferCheckEnrichmentRepository: NewTransferCheckEnrichmentRepository(
			gcsRepository,
			options.transfercheckEnrichmentBucket,
		),
	}
}
