package repositories

import (
	"crypto/rsa"

	"firebase.google.com/go/v4/auth"
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/repositories/db_connection_pool_repository"
	"github.com/checkmarble/marble-backend/repositories/firebase"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repositories struct {
	TransactionFactoryPosgresql   TransactionFactoryPosgresql
	ExecutorGetter                ExecutorGetter
	FirebaseTokenRepository       FireBaseTokenRepository
	MarbleJwtRepository           func() MarbleJwtRepository
	UserRepository                UserRepository
	OrganizationRepository        OrganizationRepository
	IngestionRepository           IngestionRepository
	DataModelRepository           DataModelRepository
	IngestedDataReadRepository    IngestedDataReadRepository
	BlankDataReadRepository       BlankDataReadRepository
	DecisionRepository            DecisionRepository
	MarbleDbRepository            MarbleDbRepository
	ScenarioPublicationRepository ScenarioPublicationRepository
	OrganizationSchemaRepository  OrganizationSchemaRepository
	AwsS3Repository               AwsS3Repository
	GcsRepository                 GcsRepository
	CustomListRepository          CustomListRepository
	UploadLogRepository           UploadLogRepository
}

func NewQueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}

func NewRepositories(
	marbleJwtSigningKey *rsa.PrivateKey,
	firebaseClient *auth.Client,
	marbleConnectionPool *pgxpool.Pool,
) (*Repositories, error) {

	databaseConnectionPoolRepository := db_connection_pool_repository.NewDatabaseConnectionPoolRepository(
		marbleConnectionPool,
	)

	transactionFactory := NewTransactionFactoryPosgresql(
		databaseConnectionPoolRepository,
		marbleConnectionPool,
	)

	executorGetter := NewExecutorGetter(marbleConnectionPool)

	return &Repositories{
		TransactionFactoryPosgresql: transactionFactory,
		ExecutorGetter:              executorGetter,
		FirebaseTokenRepository:     firebase.New(firebaseClient),
		MarbleJwtRepository: func() MarbleJwtRepository {
			if marbleJwtSigningKey == nil {
				panic("Repositories does not contain a jwt signing key")
			}
			return MarbleJwtRepository{
				jwtSigningPrivateKey: *marbleJwtSigningKey,
			}
		},
		UserRepository: &UserRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		OrganizationRepository: &OrganizationRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		IngestionRepository: &IngestionRepositoryImpl{},
		DataModelRepository: &DataModelRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		IngestedDataReadRepository: &IngestedDataReadRepositoryImpl{},
		BlankDataReadRepository:    &BlankDataReadRepositoryImpl{},
		DecisionRepository: &DecisionRepositoryImpl{
			transactionFactory: transactionFactory,
		},
		MarbleDbRepository: MarbleDbRepository{
			transactionFactory: transactionFactory,
		},
		ScenarioPublicationRepository: NewScenarioPublicationRepositoryPostgresql(
			transactionFactory,
		),
		OrganizationSchemaRepository: &OrganizationSchemaRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		CustomListRepository: &CustomListRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		UploadLogRepository: &UploadLogRepositoryImpl{
			transactionFactory: transactionFactory,
		},
		AwsS3Repository: AwsS3Repository{s3Client: NewS3Client()},
		GcsRepository:   &GcsRepositoryImpl{},
	}, nil
}
