package repositories

import (
	"crypto/rsa"
	"marble/marble-backend/pg_repository"

	"firebase.google.com/go/v4/auth"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repositories struct {
	TransactionFactory               TransactionFactory
	FirebaseTokenRepository          FireBaseTokenRepository
	MarbleJwtRepository              MarbleJwtRepository
	UserRepository                   UserRepository
	ApiKeyRepository                 ApiKeyRepository
	OrganizationRepository           OrganizationRepository
	IngestionRepository              IngestionRepository
	DataModelRepository              DataModelRepository
	DbPoolRepository                 DbPoolRepository
	IngestedDataReadRepository       IngestedDataReadRepository
	DecisionRepository               DecisionRepository
	ScenarioReadRepository           ScenarioReadRepository
	ScenarioWriteRepository          ScenarioWriteRepository
	ScenarioIterationReadRepository  ScenarioIterationReadRepository
	ScenarioIterationWriteRepository ScenarioIterationWriteRepository
	ScenarioIterationRuleRepository  ScenarioIterationRuleRepository
	ScenarioPublicationRepository    ScenarioPublicationRepository
	LegacyPgRepository               *pg_repository.PGRepository
	ClientTablesRepository           ClientTablesRepository
}

func NewRepositories(
	marbleJwtSigningKey rsa.PrivateKey,
	firebaseClient auth.Client,
	pgRepository *pg_repository.PGRepository,
	marbleConnectionPool *pgxpool.Pool,
) *Repositories {
	databaseConnectionPoolRepository := NewDatabaseConnectionPoolRepository(
		marbleConnectionPool,
	)

	transactionFactory := &TransactionFactoryPosgresql{
		databaseConnectionPoolRepository: databaseConnectionPoolRepository,
		marbleConnectionPool:             marbleConnectionPool,
	}

	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

	return &Repositories{
		TransactionFactory: transactionFactory,
		FirebaseTokenRepository: FireBaseTokenRepository{
			firebaseClient: firebaseClient,
		},
		MarbleJwtRepository: MarbleJwtRepository{
			jwtSigningPrivateKey: marbleJwtSigningKey,
		},
		UserRepository: &UserRepositoryPostgresql{
			queryBuilder: queryBuilder,
		},
		ApiKeyRepository: pgRepository,
		OrganizationRepository: &OrganizationRepositoryPostgresql{
			transactionFactory: transactionFactory,
			queryBuilder:       queryBuilder,
		},
		IngestionRepository:              pgRepository,
		DataModelRepository:              pgRepository,
		DbPoolRepository:                 pgRepository,
		IngestedDataReadRepository:       pgRepository,
		DecisionRepository:               pgRepository,
		ScenarioReadRepository:           pgRepository,
		ScenarioWriteRepository:          pgRepository,
		ScenarioIterationReadRepository:  pgRepository,
		ScenarioIterationWriteRepository: pgRepository,
		ScenarioIterationRuleRepository:  pgRepository,
		ScenarioPublicationRepository:    pgRepository,
		LegacyPgRepository:               pgRepository,
		ClientTablesRepository: &ClientTablesRepositoryPostgresql{
			queryBuilder: queryBuilder,
		},
	}
}
