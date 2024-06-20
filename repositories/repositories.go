package repositories

import (
	"firebase.google.com/go/v4/auth"
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/repositories/firebase"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repositories struct {
	ExecutorGetter                    ExecutorGetter
	FirebaseTokenRepository           FireBaseTokenRepository
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
	firebaseClient *auth.Client,
	marbleConnectionPool *pgxpool.Pool,
	metabase Metabase,
	tranfsercheckEnrichmentBucket string,
) *Repositories {
	executorGetter := NewExecutorGetter(marbleConnectionPool)

	gcsRepository := GcsRepositoryImpl{}
	return &Repositories{
		ExecutorGetter:                executorGetter,
		FirebaseTokenRepository:       firebase.New(firebaseClient),
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
		GcsRepository:                 &gcsRepository,
		MarbleAnalyticsRepository: MarbleAnalyticsRepository{
			metabase: metabase,
		},
		TransferCheckEnrichmentRepository: NewTransferCheckEnrichmentRepository(
			&gcsRepository,
			tranfsercheckEnrichmentBucket,
		),
	}
}
