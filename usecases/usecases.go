package usecases

import (
	"marble/marble-backend/repositories"
)

type Configuration struct {
	TokenLifetimeMinute int
}

type Usecases struct {
	Repositories repositories.Repositories
	Config       Configuration
}

func (usecases *Usecases) NewMarbleTokenUseCase() MarbleTokenUseCase {
	repositories := usecases.Repositories
	return MarbleTokenUseCase{
		transactionFactory:       repositories.TransactionFactory,
		firebaseTokenRepository:  repositories.FirebaseTokenRepository,
		marbleJwtRepository:      repositories.MarbleJwtRepository,
		userRepository:           repositories.UserRepository,
		hardcodedUsersRepository: repositories.HardcodedUsersRepository,
		apiKeyRepository:         repositories.ApiKeyRepository,
		organizationRepository:   repositories.OrganizationRepository,
		tokenLifetimeMinute:      usecases.Config.TokenLifetimeMinute,
	}
}

func (usecases *Usecases) NewOrganizationUseCase() OrganizationUseCase {
	return OrganizationUseCase{
		transactionFactory:     usecases.Repositories.TransactionFactory,
		organizationRepository: usecases.Repositories.OrganizationRepository,
		datamodelRepository:    usecases.Repositories.DataModelRepository,
		userRepository:         usecases.Repositories.UserRepository,
	}
}

func (usecases *Usecases) NewIngestionUseCase() IngestionUseCase {
	return IngestionUseCase{
		ingestionRepository: usecases.Repositories.IngestionRepository,
	}
}

func (usecases *Usecases) NewDecisionUsecase() DecisionUsecase {
	return DecisionUsecase{
		dbPoolRepository:                usecases.Repositories.DbPoolRepository,
		ingestedDataReadRepository:      usecases.Repositories.IngestedDataReadRepository,
		decisionRepository:              usecases.Repositories.DecisionRepository,
		datamodelRepository:             usecases.Repositories.DataModelRepository,
		scenarioReadRepository:          usecases.Repositories.ScenarioReadRepository,
		scenarioIterationReadRepository: usecases.Repositories.ScenarioIterationReadRepository,
	}
}

func (usecases *Usecases) NewUserUseCase() UserUseCase {
	return UserUseCase{
		transactionFactory: usecases.Repositories.TransactionFactory,
		userRepository:     usecases.Repositories.UserRepository,
	}
}
