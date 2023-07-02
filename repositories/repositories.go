package repositories

import (
	"crypto/rsa"
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"

	"firebase.google.com/go/v4/auth"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slog"
)

type Repositories struct {
	DatabaseConnectionPoolRepository DatabaseConnectionPoolRepository
	TransactionFactory               TransactionFactory
	FirebaseTokenRepository          FireBaseTokenRepository
	MarbleJwtRepository              MarbleJwtRepository
	UserRepository                   UserRepository
	ApiKeyRepository                 ApiKeyRepository
	OrganizationRepository           OrganizationRepository
	IngestionRepository              IngestionRepository
	DataModelRepository              DataModelRepository
	IngestedDataReadRepository       IngestedDataReadRepository
	DecisionRepositoryLegacy         DecisionRepositoryLegacy
	DecisionRepository               DecisionRepository
	ScenarioReadRepository           ScenarioReadRepository
	ScenarioWriteRepository          ScenarioWriteRepository
	ScenarioIterationReadRepository  ScenarioIterationReadRepository
	ScenarioIterationWriteRepository ScenarioIterationWriteRepository
	ScenarioIterationRuleRepository  ScenarioIterationRuleRepository
	ScenarioPublicationRepository    ScenarioPublicationRepository
	ScheduledExecutionRepository     ScheduledExecutionRepository
	LegacyPgRepository               *pg_repository.PGRepository
	OrganizationSchemaRepository     OrganizationSchemaRepository
	AwsS3Repository                  AwsS3Repository
	CustomListRepository             CustomListRepository
}

func NewQueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}

func NewRepositories(
	configuration models.GlobalConfiguration,
	marbleJwtSigningKey rsa.PrivateKey,
	firebaseClient auth.Client,
	pgRepository *pg_repository.PGRepository,
	marbleConnectionPool *pgxpool.Pool,
	appLogger *slog.Logger,
) (*Repositories, error) {

	databaseConnectionPoolRepository := NewDatabaseConnectionPoolRepository(
		marbleConnectionPool,
	)

	transactionFactory := &TransactionFactoryPosgresql{
		databaseConnectionPoolRepository: databaseConnectionPoolRepository,
		marbleConnectionPool:             marbleConnectionPool,
	}

	return &Repositories{
		DatabaseConnectionPoolRepository: databaseConnectionPoolRepository,
		TransactionFactory:               transactionFactory,
		FirebaseTokenRepository: FireBaseTokenRepository{
			firebaseClient: firebaseClient,
		},
		MarbleJwtRepository: MarbleJwtRepository{
			jwtSigningPrivateKey: marbleJwtSigningKey,
		},
		UserRepository: &UserRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		ApiKeyRepository: &ApiKeyRepositoryImpl{
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
		DecisionRepositoryLegacy:   pgRepository,
		DecisionRepository: &DecisionRepositoryImpl{
			transactionFactory: transactionFactory,
		},
		ScenarioReadRepository:           pgRepository,
		ScenarioWriteRepository:          pgRepository,
		ScenarioIterationReadRepository:  pgRepository,
		ScenarioIterationWriteRepository: pgRepository,
		ScenarioIterationRuleRepository:  pgRepository,
		ScenarioPublicationRepository:    pgRepository,
		ScheduledExecutionRepository: &ScheduledExecutionRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		LegacyPgRepository: pgRepository,
		OrganizationSchemaRepository: &OrganizationSchemaRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		CustomListRepository: &CustomListRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		AwsS3Repository: func() AwsS3Repository {
			if configuration.FakeAwsS3Repository {
				return &AwsS3RepositoryFake{}
			}

			return &AwsS3RepositoryImpl{
				s3Client: NewS3Client(),
				logger:   appLogger,
			}
		}(),
	}, nil
}
