package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/usecases/security"

	"golang.org/x/exp/slog"
)

type UsecasesWithCreds struct {
	Usecases
	Credentials             models.Credentials
	Logger                  *slog.Logger
	OrganizationIdOfContext func() (string, error)
	Context                 context.Context
}

func (usecases *UsecasesWithCreds) NewEnforceSecurity() security.EnforceSecurity {
	return &security.EnforceSecurityImpl{
		Credentials: usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceScenarioSecurity() security.EnforceSecurityScenario {
	return &security.EnforceSecurityScenarioImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioUsecase() ScenarioUsecase {
	return ScenarioUsecase{
		transactionFactory:      usecases.Repositories.TransactionFactory,
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity:         usecases.NewEnforceScenarioSecurity(),
		scenarioReadRepository:  usecases.Repositories.ScenarioReadRepository,
		scenarioWriteRepository: usecases.Repositories.ScenarioWriteRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioIterationUsecase() ScenarioIterationUsecase {
	return ScenarioIterationUsecase{
		organizationIdOfContext:                 usecases.OrganizationIdOfContext,
		scenarioIterationsReadRepository:        usecases.Repositories.ScenarioIterationReadRepository,
		scenarioIterationsWriteRepositoryLegacy: usecases.Repositories.ScenarioIterationWriteRepositoryLegacy,
		scenarioIterationsWriteRepository:       usecases.Repositories.ScenarioIterationWriteRepository,
		enforceSecurity:                         usecases.NewEnforceScenarioSecurity(),
		scenarioFetcher:                         usecases.NewScenarioFetcher(),
		validateScenarioIteration:               usecases.NewValidateScenarioIteration(),
	}
}

func (usecases *UsecasesWithCreds) NewRuleUsecase() RuleUsecase {
	return RuleUsecase{
		enforceSecurity:  usecases.NewEnforceScenarioSecurity(),
		repositoryLegacy: usecases.Repositories.ScenarioIterationRuleRepositoryLegacy,
		repository:       usecases.Repositories.RuleRepository,
		scenarioFetcher:  usecases.NewScenarioFetcher(),
	}
}

func (usecases *UsecasesWithCreds) AstExpressionUsecase() AstExpressionUsecase {
	return AstExpressionUsecase{
		EnforceSecurity:      usecases.NewEnforceSecurity(),
		CustomListRepository: usecases.Repositories.CustomListRepository,
		DataModelRepository:  usecases.Repositories.DataModelRepository,
		ScenarioRepository:   usecases.Repositories.ScenarioReadRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioPublicationUsecase() ScenarioPublicationUsecase {
	return ScenarioPublicationUsecase{
		transactionFactory:             usecases.Repositories.TransactionFactory,
		scenarioPublicationsRepository: usecases.Repositories.ScenarioPublicationRepository,
		OrganizationIdOfContext:        usecases.OrganizationIdOfContext,
		enforceSecurity:                usecases.NewEnforceScenarioSecurity(),
		scenarioFetcher:                usecases.NewScenarioFetcher(),
		scenarioPublisher:              usecases.NewScenarioPublisher(),
	}
}

func (usecases *UsecasesWithCreds) NewMarbleTokenUseCase() MarbleTokenUseCase {
	repositories := usecases.Repositories
	return MarbleTokenUseCase{
		transactionFactory:      repositories.TransactionFactory,
		firebaseTokenRepository: repositories.FirebaseTokenRepository,
		marbleJwtRepository:     repositories.MarbleJwtRepository(),
		userRepository:          repositories.UserRepository,
		apiKeyRepository:        repositories.ApiKeyRepository,
		organizationRepository:  repositories.OrganizationRepository,
		tokenLifetimeMinute:     usecases.Configuration.TokenLifetimeMinute,
		context:                 usecases.Context,
	}
}
