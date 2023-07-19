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
	OrganizationIdOfContext string
	Context                 context.Context
}

func (usecases *UsecasesWithCreds) NewEnforceSecurity() security.EnforceSecurity {
	return &security.EnforceSecurityImpl{
		Credentials: usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioUsecase() ScenarioUsecase {
	return ScenarioUsecase{
		OrganizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity:         usecases.NewEnforceSecurity(),
		scenarioReadRepository:  usecases.Repositories.ScenarioReadRepository,
		scenarioWriteRepository: usecases.Repositories.ScenarioWriteRepository,
	}
}

func (usecases *UsecasesWithCreds) AstExpressionUsecase() AstExpressionUsecase {
	return AstExpressionUsecase{
		EnforceSecurity:                 usecases.NewEnforceSecurity(),
		OrganizationIdOfContext:         usecases.OrganizationIdOfContext,
		CustomListRepository:            usecases.Repositories.CustomListRepository,
		OrgTransactionFactory:           usecases.NewOrgTransactionFactory(),
		IngestedDataReadRepository:      usecases.Repositories.IngestedDataReadRepository,
		DataModelRepository:             usecases.Repositories.DataModelRepository,
		ScenarioRepository:              usecases.Repositories.ScenarioReadRepository,
		ScenarioIterationReadRepository: usecases.Repositories.ScenarioIterationReadRepository,
		RuleRepository:                  usecases.Repositories.RuleRepository,
		ScenarioIterationRuleUsecase:    usecases.Repositories.ScenarioIterationRuleRepositoryLegacy,
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
