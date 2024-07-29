package repositories

import (
	"firebase.google.com/go/v4/auth"
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/repositories/firebase"
	"github.com/jackc/pgx/v5/pgxpool"
)

type options struct {
	firebaseClient                *auth.Client
	metabase                      Metabase
	transfercheckEnrichmentBucket string
	fakeGcsRepository             bool
	convoyClientProvider          ConvoyClientProvider
}

type Option func(*options)

func getOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithFirebaseClient(firebaseClient *auth.Client) Option {
	return func(o *options) {
		o.firebaseClient = firebaseClient
	}
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

func WithConvoyClientProvider(convoyResources ConvoyClientProvider) Option {
	return func(o *options) {
		o.convoyClientProvider = convoyResources
	}
}

type Repositories struct {
	ExecutorGetter                    ExecutorGetter
	FirebaseTokenRepository           FireBaseTokenRepository
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
		FirebaseTokenRepository:       firebase.New(options.firebaseClient),
		ConvoyRepository:              NewConvoyRepository(options.convoyClientProvider),
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
