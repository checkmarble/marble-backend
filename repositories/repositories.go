package repositories

import (
	"crypto/rsa"

	"firebase.google.com/go/v4/auth"
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/repositories/firebase"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repositories struct {
	ExecutorGetter                ExecutorGetter
	FirebaseTokenRepository       FireBaseTokenRepository
	MarbleJwtRepository           func() MarbleJwtRepository
	UserRepository                UserRepository
	OrganizationRepository        OrganizationRepository
	IngestionRepository           IngestionRepository
	DataModelRepository           DataModelRepository
	IngestedDataReadRepository    IngestedDataReadRepository
	DecisionRepository            DecisionRepository
	MarbleDbRepository            MarbleDbRepository
	ClientDbRepository            ClientDbRepository
	ScenarioPublicationRepository ScenarioPublicationRepository
	OrganizationSchemaRepository  OrganizationSchemaRepository
	AwsS3Repository               AwsS3Repository
	GcsRepository                 GcsRepository
	CustomListRepository          CustomListRepository
	UploadLogRepository           UploadLogRepository
	MarbleAnalyticsRepository     MarbleAnalyticsRepository
}

func NewQueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}

func NewRepositories(
	marbleJwtSigningKey *rsa.PrivateKey,
	firebaseClient *auth.Client,
	marbleConnectionPool *pgxpool.Pool,
	metabase Metabase,
) (*Repositories, error) {

	executorGetter := NewExecutorGetter(marbleConnectionPool)

	return &Repositories{
		ExecutorGetter:          executorGetter,
		FirebaseTokenRepository: firebase.New(firebaseClient),
		MarbleJwtRepository: func() MarbleJwtRepository {
			if marbleJwtSigningKey == nil {
				panic("Repositories does not contain a jwt signing key")
			}
			return MarbleJwtRepository{
				jwtSigningPrivateKey: *marbleJwtSigningKey,
			}
		},
		UserRepository: &UserRepositoryPostgresql{
			executorGetter: executorGetter,
		},
		OrganizationRepository: &OrganizationRepositoryPostgresql{
			executorGetter: executorGetter,
		},
		IngestionRepository: &IngestionRepositoryImpl{},
		DataModelRepository: &DataModelRepositoryPostgresql{
			executorGetter: executorGetter,
		},
		IngestedDataReadRepository: &IngestedDataReadRepositoryImpl{},
		DecisionRepository: &DecisionRepositoryImpl{
			executorGetter: executorGetter,
		},
		MarbleDbRepository: MarbleDbRepository{
			executorGetter: executorGetter,
		},
		ClientDbRepository: ClientDbRepository{},
		ScenarioPublicationRepository: &ScenarioPublicationRepositoryPostgresql{
			executorGetter: executorGetter,
		},
		OrganizationSchemaRepository: &OrganizationSchemaRepositoryPostgresql{
			executorGetter: executorGetter,
		},
		CustomListRepository: &CustomListRepositoryPostgresql{
			executorGetter: executorGetter,
		},
		UploadLogRepository: &UploadLogRepositoryImpl{
			executorGetter: executorGetter,
		},
		AwsS3Repository: AwsS3Repository{s3Client: NewS3Client()},
		GcsRepository:   &GcsRepositoryImpl{},
		MarbleAnalyticsRepository: MarbleAnalyticsRepository{
			metabase: metabase,
		},
	}, nil
}
