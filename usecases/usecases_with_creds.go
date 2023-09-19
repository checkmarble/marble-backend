package usecases

import (
	"context"
	"log/slog"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/scheduledexecution"
	"github.com/checkmarble/marble-backend/usecases/security"
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

func (usecases *UsecasesWithCreds) NewEnforceDecisionSecurity() security.EnforceSecurityDecision {
	return &security.EnforceSecurityDecisionImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceCustomListSecurity() security.EnforceSecurityCustomList {
	return &security.EnforceSecurityCustomListImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceOrganizationSecurity() security.EnforceSecurityOrganization {
	return &security.EnforceSecurityOrganizationImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceIngestionSecurity() security.EnforceSecurityIngestion {
	return &security.EnforceSecurityIngestionImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceAdminSecurity() security.EnforceSecurityAdmin {
	return &security.EnforceSecurityAdminImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}
func (usecases *UsecasesWithCreds) NewDecisionUsecase() DecisionUsecase {
	return DecisionUsecase{
		enforceSecurity:                 usecases.NewEnforceDecisionSecurity(),
		transactionFactory:              &usecases.Repositories.TransactionFactoryPosgresql,
		orgTransactionFactory:           usecases.NewOrgTransactionFactory(),
		ingestedDataReadRepository:      usecases.Repositories.IngestedDataReadRepository,
		decisionRepository:              usecases.Repositories.DecisionRepository,
		datamodelRepository:             usecases.Repositories.DataModelRepository,
		scenarioReadRepository:          usecases.Repositories.ScenarioReadRepository,
		scenarioIterationReadRepository: usecases.Repositories.ScenarioIterationReadRepository,
		customListRepository:            usecases.Repositories.CustomListRepository,
		evaluateRuleAstExpression:       usecases.NewEvaluateRuleAstExpression(),
	}
}

func (usecases *UsecasesWithCreds) NewScenarioUsecase() ScenarioUsecase {
	return ScenarioUsecase{
		transactionFactory:      &usecases.Repositories.TransactionFactoryPosgresql,
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity:         usecases.NewEnforceScenarioSecurity(),
		scenarioReadRepository:  usecases.Repositories.ScenarioReadRepository,
		scenarioWriteRepository: usecases.Repositories.ScenarioWriteRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioIterationUsecase() ScenarioIterationUsecase {
	return ScenarioIterationUsecase{
		organizationIdOfContext:           usecases.OrganizationIdOfContext,
		scenarioIterationsReadRepository:  usecases.Repositories.ScenarioIterationReadRepository,
		scenarioIterationsWriteRepository: usecases.Repositories.ScenarioIterationWriteRepository,
		enforceSecurity:                   usecases.NewEnforceScenarioSecurity(),
		scenarioFetcher:                   usecases.NewScenarioFetcher(),
		validateScenarioIteration:         usecases.NewValidateScenarioIteration(),
	}
}

func (usecases *UsecasesWithCreds) NewRuleUsecase() RuleUsecase {
	return RuleUsecase{
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity:         usecases.NewEnforceScenarioSecurity(),
		repository:              usecases.Repositories.RuleRepository,
		scenarioFetcher:         usecases.NewScenarioFetcher(),
		transactionFactory:      &usecases.Repositories.TransactionFactoryPosgresql,
	}
}

func (usecases *UsecasesWithCreds) AstExpressionUsecase() AstExpressionUsecase {
	return AstExpressionUsecase{
		EnforceSecurity:     usecases.NewEnforceScenarioSecurity(),
		DataModelRepository: usecases.Repositories.DataModelRepository,
		ScenarioRepository:  usecases.Repositories.ScenarioReadRepository,
	}
}

func (usecases *UsecasesWithCreds) NewCustomListUseCase() CustomListUseCase {
	return CustomListUseCase{
		enforceSecurity:         usecases.NewEnforceCustomListSecurity(),
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		transactionFactory:      &usecases.Repositories.TransactionFactoryPosgresql,
		CustomListRepository:    usecases.Repositories.CustomListRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioPublicationUsecase() ScenarioPublicationUsecase {
	return ScenarioPublicationUsecase{
		transactionFactory:             &usecases.Repositories.TransactionFactoryPosgresql,
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
		transactionFactory:      &usecases.Repositories.TransactionFactoryPosgresql,
		firebaseTokenRepository: repositories.FirebaseTokenRepository,
		marbleJwtRepository:     repositories.MarbleJwtRepository(),
		userRepository:          repositories.UserRepository,
		apiKeyRepository:        repositories.ApiKeyRepository,
		organizationRepository:  repositories.OrganizationRepository,
		tokenLifetimeMinute:     usecases.Configuration.TokenLifetimeMinute,
		context:                 usecases.Context,
	}
}

func (usecases *UsecasesWithCreds) NewOrganizationUseCase() OrganizationUseCase {
	return OrganizationUseCase{
		enforceSecurity:              usecases.NewEnforceOrganizationSecurity(),
		transactionFactory:           &usecases.Repositories.TransactionFactoryPosgresql,
		orgTransactionFactory:        usecases.NewOrgTransactionFactory(),
		organizationRepository:       usecases.Repositories.OrganizationRepository,
		datamodelRepository:          usecases.Repositories.DataModelRepository,
		apiKeyRepository:             usecases.Repositories.ApiKeyRepository,
		userRepository:               usecases.Repositories.UserRepository,
		organizationCreator:          usecases.NewOrganizationCreator(),
		organizationSchemaRepository: usecases.Repositories.OrganizationSchemaRepository,
		populateOrganizationSchema:   usecases.NewPopulateOrganizationSchema(),
	}
}

func (usecases *UsecasesWithCreds) NewIngestionUseCase() IngestionUseCase {
	return IngestionUseCase{
		enforceSecurity:       usecases.NewEnforceIngestionSecurity(),
		orgTransactionFactory: usecases.NewOrgTransactionFactory(),
		ingestionRepository:   usecases.Repositories.IngestionRepository,
		gcsRepository:         usecases.Repositories.GcsRepository,
		datamodelRepository:   usecases.Repositories.DataModelRepository,
	}
}

func (usecases *UsecasesWithCreds) NewRunScheduledExecution() scheduledexecution.RunScheduledExecution {
	return scheduledexecution.RunScheduledExecution{
		ScenarioReadRepository:          usecases.Repositories.ScenarioReadRepository,
		TransactionFactory:              &usecases.Repositories.TransactionFactoryPosgresql,
		ScheduledExecutionRepository:    usecases.Repositories.ScheduledExecutionRepository,
		ExportScheduleExecution:         *usecases.NewExportScheduleExecution(),
		ScenarioPublicationsRepository:  usecases.Repositories.ScenarioPublicationRepository,
		DataModelRepository:             usecases.Repositories.DataModelRepository,
		OrgTransactionFactory:           usecases.NewOrgTransactionFactory(),
		IngestedDataReadRepository:      usecases.Repositories.IngestedDataReadRepository,
		ScenarioIterationReadRepository: usecases.Repositories.ScenarioIterationReadRepository,
		EvaluateRuleAstExpression:       usecases.NewEvaluateRuleAstExpression(),
		DecisionRepository:              usecases.Repositories.DecisionRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScheduledExecutionUsecase() ScheduledExecutionUsecase {
	return ScheduledExecutionUsecase{
		enforceSecurity:              usecases.NewEnforceDecisionSecurity(),
		transactionFactory:           &usecases.Repositories.TransactionFactoryPosgresql,
		scheduledExecutionRepository: usecases.Repositories.ScheduledExecutionRepository,
		exportScheduleExecution:      usecases.NewExportScheduleExecution(),
		organizationIdOfContext:      usecases.OrganizationIdOfContext,
	}
}

func (usecases *UsecasesWithCreds) NewUserUseCase() UserUseCase {
	return UserUseCase{
		enforceAdminSecurity: usecases.NewEnforceAdminSecurity(),
		transactionFactory:   &usecases.Repositories.TransactionFactoryPosgresql,
		userRepository:       usecases.Repositories.UserRepository,
	}
}
