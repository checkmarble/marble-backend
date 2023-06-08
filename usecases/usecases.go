package usecases

import (
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"
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
		transactionFactory:      repositories.TransactionFactory,
		firebaseTokenRepository: repositories.FirebaseTokenRepository,
		marbleJwtRepository:     repositories.MarbleJwtRepository,
		userRepository:          repositories.UserRepository,
		apiKeyRepository:        repositories.ApiKeyRepository,
		organizationRepository:  repositories.OrganizationRepository,
		tokenLifetimeMinute:     usecases.Config.TokenLifetimeMinute,
	}
}

func (usecases *Usecases) NewOrganizationUseCase() OrganizationUseCase {
	return OrganizationUseCase{
		transactionFactory:     usecases.Repositories.TransactionFactory,
		organizationRepository: usecases.Repositories.OrganizationRepository,
		datamodelRepository:    usecases.Repositories.DataModelRepository,
		apiKeyRepository:       usecases.Repositories.ApiKeyRepository,
		userRepository:         usecases.Repositories.UserRepository,
		organizationCreator:    usecases.NewOrganizationCreator(),
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

func (usecases *Usecases) NewSeedUseCase() SeedUseCase {
	return SeedUseCase{
		transactionFactory:  usecases.Repositories.TransactionFactory,
		userRepository:      usecases.Repositories.UserRepository,
		organizationCreator: usecases.NewOrganizationCreator(),
	}
}

func (usecases *Usecases) NewOrganizationCreator() organization.OrganizationCreator {
	return organization.OrganizationCreator{
		TransactionFactory:     usecases.Repositories.TransactionFactory,
		OrganizationRepository: usecases.Repositories.OrganizationRepository,
		OrganizationSeeder:     usecases.Repositories.LegacyPgRepository,
		PopulateClientTables: organization.PopulateClientTables{
			TransactionFactory:     usecases.Repositories.TransactionFactory,
			OrganizationRepository: usecases.Repositories.OrganizationRepository,
			ClientTablesRepository: usecases.Repositories.ClientTablesRepository,
		},
	}
}

func (usecases *Usecases) NewScenarioUsecase() ScenarioUsecase {
	return ScenarioUsecase{
		scenarioReadRepository:  usecases.Repositories.ScenarioReadRepository,
		scenarioWriteRepository: usecases.Repositories.ScenarioWriteRepository,
	}
}

func (usecases *Usecases) NewScenarioPublicationUsecase() ScenarioPublicationUsecase {
	return ScenarioPublicationUsecase{
		scenarioPublicationsRepository: usecases.Repositories.ScenarioPublicationRepository,
	}
}

func (usecases *Usecases) NewScenarioIterationUsecase() ScenarioIterationUsecase {
	return ScenarioIterationUsecase{
		scenarioIterationsReadRepository:  usecases.Repositories.ScenarioIterationReadRepository,
		scenarioIterationsWriteRepository: usecases.Repositories.ScenarioIterationWriteRepository,
	}
}

func (usecases *Usecases) NewScenarioIterationRuleUsecase() ScenarioIterationRuleUsecase {
	return ScenarioIterationRuleUsecase{
		repository: usecases.Repositories.ScenarioIterationRuleRepository,
	}
}
