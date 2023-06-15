package usecases

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"
	"marble/marble-backend/usecases/scheduledexecution"
)

type Usecases struct {
	Repositories  repositories.Repositories
	Configuration models.GlobalConfiguration
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
		tokenLifetimeMinute:     usecases.Configuration.TokenLifetimeMinute,
	}
}

func (usecases *Usecases) NewOrganizationUseCase() OrganizationUseCase {
	return OrganizationUseCase{
		transactionFactory:           usecases.Repositories.TransactionFactory,
		orgTransactionFactory:        usecases.NewOrgTransactionFactory(),
		organizationRepository:       usecases.Repositories.OrganizationRepository,
		datamodelRepository:          usecases.Repositories.DataModelRepository,
		apiKeyRepository:             usecases.Repositories.ApiKeyRepository,
		userRepository:               usecases.Repositories.UserRepository,
		organizationCreator:          usecases.NewOrganizationCreator(),
		organizationSchemaRepository: usecases.Repositories.OrganizationSchemaRepository,
	}
}

func (usecases *Usecases) NewOrgTransactionFactory() organization.OrgTransactionFactory {
	return &organization.OrgTransactionFactoryImpl{
		OrganizationSchemaRepository:     usecases.Repositories.OrganizationSchemaRepository,
		TransactionFactory:               usecases.Repositories.TransactionFactory,
		DatabaseConnectionPoolRepository: usecases.Repositories.DatabaseConnectionPoolRepository,
	}
}

func (usecases *Usecases) NewIngestionUseCase() IngestionUseCase {
	return IngestionUseCase{
		orgTransactionFactory: usecases.NewOrgTransactionFactory(),
		ingestionRepository:   usecases.Repositories.IngestionRepository,
	}
}

func (usecases *Usecases) NewDecisionUsecase() DecisionUsecase {
	return DecisionUsecase{
		orgTransactionFactory:           usecases.NewOrgTransactionFactory(),
		ingestedDataReadRepository:      usecases.Repositories.IngestedDataReadRepository,
		decisionRepositoryLegacy:        usecases.Repositories.DecisionRepositoryLegacy,
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
		transactionFactory:     usecases.Repositories.TransactionFactory,
		userRepository:         usecases.Repositories.UserRepository,
		organizationCreator:    usecases.NewOrganizationCreator(),
		organizationRepository: usecases.Repositories.OrganizationRepository,
	}
}

func (usecases *Usecases) NewOrganizationCreator() organization.OrganizationCreator {
	return organization.OrganizationCreator{
		TransactionFactory:     usecases.Repositories.TransactionFactory,
		OrganizationRepository: usecases.Repositories.OrganizationRepository,
		OrganizationSeeder:     usecases.Repositories.LegacyPgRepository,
		PopulateOrganizationSchema: organization.PopulateOrganizationSchema{
			TransactionFactory:           usecases.Repositories.TransactionFactory,
			OrganizationRepository:       usecases.Repositories.OrganizationRepository,
			OrganizationSchemaRepository: usecases.Repositories.OrganizationSchemaRepository,
			DataModelRepository:          usecases.Repositories.DataModelRepository,
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

func (usecases *Usecases) NewScheduledExecutionUsecase() ScheduledExecutionUsecase {
	return ScheduledExecutionUsecase{
		scenarioReadRepository:          usecases.Repositories.ScenarioReadRepository,
		scenarioIterationReadRepository: usecases.Repositories.ScenarioIterationReadRepository,
		scheduledExecutionRepository:    usecases.Repositories.ScheduledExecutionRepository,
		transactionFactory:              usecases.Repositories.TransactionFactory,
		dataModelRepository:             usecases.Repositories.DataModelRepository,
		ingestedDataReadRepository:      usecases.Repositories.IngestedDataReadRepository,
	}
}

func (usecases *Usecases) NewExportScheduleExecution() scheduledexecution.ExportScheduleExecution {
	return &scheduledexecution.ExportScheduleExecutionImpl{
		AwsS3Repository:    usecases.Repositories.AwsS3Repository,
		DecisionRepository: usecases.Repositories.DecisionRepository,
	}
}
