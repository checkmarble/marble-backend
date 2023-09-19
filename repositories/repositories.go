package repositories

import (
	"crypto/rsa"
	"log/slog"

	"firebase.google.com/go/v4/auth"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repositories struct {
	DatabaseConnectionPoolRepository DatabaseConnectionPoolRepository
	TransactionFactoryPosgresql      TransactionFactoryPosgresql
	FirebaseTokenRepository          FireBaseTokenRepository
	MarbleJwtRepository              func() MarbleJwtRepository
	UserRepository                   UserRepository
	ApiKeyRepository                 ApiKeyRepository
	OrganizationRepository           OrganizationRepository
	IngestionRepository              IngestionRepository
	DataModelRepository              DataModelRepository
	IngestedDataReadRepository       IngestedDataReadRepository
	BlankDataReadRepository          BlankDataReadRepository
	DecisionRepository               DecisionRepository
	RuleRepository                   RuleRepository
	ScenarioReadRepository           ScenarioReadRepository
	ScenarioWriteRepository          ScenarioWriteRepository
	ScenarioIterationReadRepository  ScenarioIterationReadRepository
	ScenarioIterationWriteRepository ScenarioIterationWriteRepository
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
	marbleJwtSigningKey *rsa.PrivateKey,
	firebaseClient *auth.Client,
	marbleConnectionPool *pgxpool.Pool,
	appLogger *slog.Logger,

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
		TransactionFactoryPosgresql:      transactionFactory,
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
		BlankDataReadRepository:    &BlankDataReadRepositoryImpl{},
		DecisionRepository: &DecisionRepositoryImpl{
			transactionFactory: transactionFactory,
		},
		RuleRepository: &RuleRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		ScenarioReadRepository: NewScenarioReadRepositoryPostgresql(
			transactionFactory,
		),
		ScenarioWriteRepository: NewScenarioWriteRepositoryPostgresql(
			transactionFactory,
		),
		ScenarioIterationReadRepository: &ScenarioIterationReadRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		ScenarioIterationWriteRepository: &ScenarioIterationWriteRepositoryPostgresql{
			transactionFactory: transactionFactory,
			ruleRepository: &RuleRepositoryPostgresql{
				transactionFactory: transactionFactory,
			},
		},
		ScenarioPublicationRepository: NewScenarioPublicationRepositoryPostgresql(
			transactionFactory,
		),
		ScheduledExecutionRepository: &ScheduledExecutionRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		OrganizationSchemaRepository: &OrganizationSchemaRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		CustomListRepository: &CustomListRepositoryPostgresql{
			transactionFactory: transactionFactory,
		},
		AwsS3Repository: AwsS3Repository{
			s3Client: NewS3Client(),
			logger:   appLogger,
		},
		GcsRepository: &GcsRepositoryImpl{logger: appLogger},
	}, nil
}
