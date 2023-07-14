package usecases

import (
	"marble/marble-backend/models"
	"marble/marble-backend/usecases/security"

	"golang.org/x/exp/slog"
)

type UsecasesWithCreds struct {
	Usecases
	Credentials             models.Credentials
	Logger                  *slog.Logger
	OrganizationIdOfContext string
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
		EnforceSecurity:            usecases.NewEnforceSecurity(),
		OrganizationIdOfContext:    usecases.OrganizationIdOfContext,
		CustomListRepository:       usecases.Repositories.CustomListRepository,
		OrgTransactionFactory:      usecases.NewOrgTransactionFactory(),
		IngestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
		DataModelRepository:        usecases.Repositories.DataModelRepository,
		ScenarioRepository:         usecases.Repositories.ScenarioReadRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioPublicationUsecase() ScenarioPublicationUsecase {
	return ScenarioPublicationUsecase{
		transactionFactory:              usecases.Repositories.TransactionFactory,
		scenarioPublicationsRepository:  usecases.Repositories.ScenarioPublicationRepository,
		OrganizationIdOfContext:         usecases.OrganizationIdOfContext,
		scenarioReadRepository:          usecases.Repositories.ScenarioReadRepository,
		scenarioWriteRepository:         usecases.Repositories.ScenarioWriteRepository,
		scenarioIterationReadRepository: usecases.Repositories.ScenarioIterationReadRepository,
		enforceSecurity:                 usecases.NewEnforceScenarioSecurity(),
	}
}
