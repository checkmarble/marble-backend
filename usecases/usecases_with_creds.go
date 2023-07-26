package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/usecases/scenarios"
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
		OrganizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity:         usecases.NewEnforceScenarioSecurity(),
		scenarioReadRepository:  usecases.Repositories.ScenarioReadRepository,
		scenarioWriteRepository: usecases.Repositories.ScenarioWriteRepository,
	}
}

func (usecases *UsecasesWithCreds) AstExpressionUsecase() AstExpressionUsecase {
	return AstExpressionUsecase{
		EnforceSecurity:                       usecases.NewEnforceSecurity(),
		OrganizationIdOfContext:               usecases.OrganizationIdOfContext,
		CustomListRepository:                  usecases.Repositories.CustomListRepository,
		OrgTransactionFactory:                 usecases.NewOrgTransactionFactory(),
		IngestedDataReadRepository:            usecases.Repositories.IngestedDataReadRepository,
		DataModelRepository:                   usecases.Repositories.DataModelRepository,
		ScenarioRepository:                    usecases.Repositories.ScenarioReadRepository,
		ScenarioIterationReadLegacyRepository: usecases.Repositories.ScenarioIterationReadLegacyRepository,
		RuleRepository:                        usecases.Repositories.RuleRepository,
		ScenarioIterationRuleUsecase:          usecases.Repositories.ScenarioIterationRuleRepositoryLegacy,
		AstEvaluationEnvironmentFactory:       usecases.AstEvaluationEnvironment,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioPublicationUsecase() ScenarioPublicationUsecase {
	return ScenarioPublicationUsecase{
		transactionFactory:              usecases.Repositories.TransactionFactory,
		scenarioPublicationsRepository:  usecases.Repositories.ScenarioPublicationRepository,
		scenarioReadRepository:          usecases.Repositories.ScenarioReadRepository,
		scenarioIterationReadRepository: usecases.Repositories.ScenarioIterationReadLegacyRepository,
		OrganizationIdOfContext:         usecases.OrganizationIdOfContext,
		enforceSecurity:                 usecases.NewEnforceScenarioSecurity(),
		scenarioPublisher: scenarios.NewScenarioPublisher(
			usecases.Repositories.ScenarioPublicationRepository,
			usecases.Repositories.ScenarioReadRepository,
			usecases.Repositories.ScenarioWriteRepository,
			usecases.Repositories.ScenarioIterationReadLegacyRepository,
		),
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
