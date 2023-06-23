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

	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

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
			queryBuilder:       queryBuilder,
		},
		ApiKeyRepository: pgRepository,
		OrganizationRepository: &OrganizationRepositoryPostgresql{
			transactionFactory: transactionFactory,
			queryBuilder:       queryBuilder,
		},
		IngestionRepository: &IngestionRepositoryImpl{
			queryBuilder: queryBuilder,
		},
		DataModelRepository:        pgRepository,
		IngestedDataReadRepository: &IngestedDataReadRepositoryImpl{queryBuilder: queryBuilder},
		DecisionRepositoryLegacy:   pgRepository,
		DecisionRepository: &DecisionRepositoryImpl{
			transactionFactory: transactionFactory,
			queryBuilder:       queryBuilder,
		},
		ScenarioReadRepository:           pgRepository,
		ScenarioWriteRepository:          pgRepository,
		ScenarioIterationReadRepository:  pgRepository,
		ScenarioIterationWriteRepository: pgRepository,
		ScenarioIterationRuleRepository:  pgRepository,
		ScenarioPublicationRepository:    pgRepository,
		ScheduledExecutionRepository: &ScheduledExecutionRepositoryPostgresql{
			transactionFactory: transactionFactory,
			queryBuilder:       queryBuilder,
		},
		LegacyPgRepository: pgRepository,
		OrganizationSchemaRepository: &OrganizationSchemaRepositoryPostgresql{
			transactionFactory: transactionFactory,
			queryBuilder:       queryBuilder,
		},
		CustomListRepository: &CustomListRepositoryPostgresql{
			transactionFactory: transactionFactory,
			queryBuilder:       queryBuilder,
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
