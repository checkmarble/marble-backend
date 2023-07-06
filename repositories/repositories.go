package repositories

import (
	"crypto/rsa"
	"marble/marble-backend/models"

	"firebase.google.com/go/v4/auth"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slog"
)

type Repositories struct {
	DatabaseConnectionPoolRepository DatabaseConnectionPoolRepository
	TransactionFactory               TransactionFactory
	FirebaseTokenRepository          FireBaseTokenRepository
	MarbleJwtRepository              func() MarbleJwtRepository
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
	OrganizationSchemaRepository     OrganizationSchemaRepository
	AwsS3Repository                  AwsS3Repository
	GcsRepository                    GcsRepository
	CustomListRepository             CustomListRepository
}

func NewQueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}

func NewRepositories(
	configuration models.GlobalConfiguration,
	marbleJwtSigningKey *rsa.PrivateKey,
	firebaseClient auth.Client,
	marbleConnectionPool *pgxpool.Pool,
	appLogger *slog.Logger,
	decisionRepositoryLegacy DecisionRepositoryLegacy,
	scenarioWriteRepository ScenarioWriteRepository,
	scenarioIterationReadRepository ScenarioIterationReadRepository,
	scenarioIterationWriteRepository ScenarioIterationWriteRepository,
	scenarioIterationRuleRepository ScenarioIterationRuleRepository,
	scenarioPublicationRepository ScenarioPublicationRepository,

) (*Repositories, error) {

	databaseConnectionPoolRepository := NewDatabaseConnectionPoolRepository(
		marbleConnectionPool,
	)

	transactionFactory := NewTransactionFactoryPosgresql(
		databaseConnectionPoolRepository,
		marbleConnectionPool,
	)

	return &Repositories{
		DatabaseConnectionPoolRepository: databaseConnectionPoolRepository,
		TransactionFactory:               transactionFactory,
		FirebaseTokenRepository: FireBaseTokenRepository{
			firebaseClient: firebaseClient,
		},
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
		DecisionRepositoryLegacy:   decisionRepositoryLegacy,
		DecisionRepository: &DecisionRepositoryImpl{
			transactionFactory: transactionFactory,
		},
		ScenarioReadRepository: NewScenarioReadRepositoryPostgresql(
			transactionFactory,
		),
		ScenarioWriteRepository:          scenarioWriteRepository,
		ScenarioIterationReadRepository:  scenarioIterationReadRepository,
		ScenarioIterationWriteRepository: scenarioIterationWriteRepository,
		ScenarioIterationRuleRepository:  scenarioIterationRuleRepository,
		ScenarioPublicationRepository:    scenarioPublicationRepository,
		ScheduledExecutionRepository: &ScheduledExecutionRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
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
		GcsRepository: &GcsRepositoryImpl{
			gcsClient: NewGCSClient(),
			logger:    appLogger,
		},
	}, nil
}
