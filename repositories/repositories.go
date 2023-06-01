package repositories

import (
	"crypto/rsa"
	. "marble/marble-backend/models"
	"marble/marble-backend/pg_repository"

	"firebase.google.com/go/v4/auth"
)

type Repositories struct {
	FirebaseTokenRepository         FireBaseTokenRepository
	MarbleJwtRepository             MarbleJwtRepository
	UserRepository                  UserRepository
	ApiKeyRepository                ApiKeyRepository
	OrganizationRepository          OrganizationRepository
	IngestionRepository             IngestionRepository
	DataModelRepository             DataModelRepository
	DbPoolRepository                DbPoolRepository
	IngestedDataReadRepository      IngestedDataReadRepository
	DecisionRepository              DecisionRepository
	ScenarioReadRepository          ScenarioReadRepository
	ScenarioIterationReadRepository ScenarioIterationReadRepository
}

func NewRepositories(marbleJwtSigningKey rsa.PrivateKey, firebaseClient auth.Client, users []User, pgRepository *pg_repository.PGRepository) *Repositories {
	return &Repositories{
		FirebaseTokenRepository: FireBaseTokenRepository{
			firebaseClient: firebaseClient,
		},
		MarbleJwtRepository: MarbleJwtRepository{
			jwtSigningPrivateKey: marbleJwtSigningKey,
		},
		UserRepository:                  NewHardcodedUserRepository(users),
		ApiKeyRepository:                pgRepository,
		OrganizationRepository:          pgRepository,
		IngestionRepository:             pgRepository,
		DataModelRepository:             pgRepository,
		DbPoolRepository:                pgRepository,
		IngestedDataReadRepository:      pgRepository,
		DecisionRepository:              pgRepository,
		ScenarioReadRepository:          pgRepository,
		ScenarioIterationReadRepository: pgRepository,
	}
}
