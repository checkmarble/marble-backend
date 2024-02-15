package usecases

import (
	"context"
	"log/slog"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
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

func (usecases *UsecasesWithCreds) NewEnforceUserSecurity() security.EnforceSecurityUser {
	return &security.EnforceSecurityUserImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceCaseSecurity() security.EnforceSecurityCase {
	return &security.EnforceSecurityCaseImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewDecisionUsecase() DecisionUsecase {
	return DecisionUsecase{
		enforceSecurity:            usecases.NewEnforceDecisionSecurity(),
		enforceSecurityScenario:    usecases.NewEnforceScenarioSecurity(),
		executorFactory:            usecases.NewExecutorFactory(),
		transactionFactory:         usecases.NewTransactionFactory(),
		ingestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
		decisionRepository:         usecases.Repositories.DecisionRepository,
		datamodelRepository:        usecases.Repositories.DataModelRepository,
		repository:                 &usecases.Repositories.MarbleDbRepository,
		evaluateRuleAstExpression:  usecases.NewEvaluateRuleAstExpression(),
		organizationIdOfContext:    usecases.OrganizationIdOfContext,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioUsecase() ScenarioUsecase {
	return ScenarioUsecase{
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity:         usecases.NewEnforceScenarioSecurity(),
		repository:              &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioIterationUsecase() ScenarioIterationUsecase {
	return ScenarioIterationUsecase{
		repository:                &usecases.Repositories.MarbleDbRepository,
		organizationIdOfContext:   usecases.OrganizationIdOfContext,
		enforceSecurity:           usecases.NewEnforceScenarioSecurity(),
		scenarioFetcher:           usecases.NewScenarioFetcher(),
		validateScenarioIteration: usecases.NewValidateScenarioIteration(),
		executorFactory:           usecases.NewExecutorFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewRuleUsecase() RuleUsecase {
	return RuleUsecase{
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity:         usecases.NewEnforceScenarioSecurity(),
		repository:              &usecases.Repositories.MarbleDbRepository,
		scenarioFetcher:         usecases.NewScenarioFetcher(),
		transactionFactory:      usecases.NewTransactionFactory(),
	}
}

func (usecases *UsecasesWithCreds) AstExpressionUsecase() AstExpressionUsecase {
	return AstExpressionUsecase{
		EnforceSecurity:     usecases.NewEnforceScenarioSecurity(),
		DataModelRepository: usecases.Repositories.DataModelRepository,
		Repository:          &usecases.Repositories.MarbleDbRepository,
		executorFactory:     usecases.NewExecutorFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewCustomListUseCase() CustomListUseCase {
	return CustomListUseCase{
		enforceSecurity:         usecases.NewEnforceCustomListSecurity(),
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		CustomListRepository:    usecases.Repositories.CustomListRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioPublicationUsecase() ScenarioPublicationUsecase {
	return ScenarioPublicationUsecase{
		transactionFactory:             usecases.NewTransactionFactory(),
		executorFactory:                usecases.NewExecutorFactory(),
		scenarioPublicationsRepository: usecases.Repositories.ScenarioPublicationRepository,
		OrganizationIdOfContext:        usecases.OrganizationIdOfContext,
		enforceSecurity:                usecases.NewEnforceScenarioSecurity(),
		scenarioFetcher:                usecases.NewScenarioFetcher(),
		scenarioPublisher:              usecases.NewScenarioPublisher(),
		scenarioListRepository:         &usecases.Repositories.MarbleDbRepository,
		ingestedDataIndexesRepository:  &usecases.Repositories.ClientDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewMarbleTokenUseCase() MarbleTokenUseCase {
	repositories := usecases.Repositories
	return MarbleTokenUseCase{
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		firebaseTokenRepository: repositories.FirebaseTokenRepository,
		marbleJwtRepository:     repositories.MarbleJwtRepository(),
		userRepository:          repositories.UserRepository,
		apiKeyRepository:        &usecases.Repositories.MarbleDbRepository,
		organizationRepository:  repositories.OrganizationRepository,
		tokenLifetimeMinute:     usecases.Configuration.TokenLifetimeMinute,
		context:                 usecases.Context,
	}
}

func (usecases *UsecasesWithCreds) NewOrganizationUseCase() OrganizationUseCase {
	return OrganizationUseCase{
		enforceSecurity:              usecases.NewEnforceOrganizationSecurity(),
		executorFactory:              usecases.NewExecutorFactory(),
		transactionFactory:           usecases.NewTransactionFactory(),
		organizationRepository:       usecases.Repositories.OrganizationRepository,
		datamodelRepository:          usecases.Repositories.DataModelRepository,
		userRepository:               usecases.Repositories.UserRepository,
		organizationCreator:          usecases.NewOrganizationCreator(),
		organizationSchemaRepository: usecases.Repositories.OrganizationSchemaRepository,
		populateOrganizationSchema:   usecases.NewPopulateOrganizationSchema(),
	}
}

func (usecases *UsecasesWithCreds) NewDataModelUseCase() DataModelUseCase {
	return DataModelUseCase{
		enforceSecurity:            usecases.NewEnforceOrganizationSecurity(),
		transactionFactory:         usecases.NewTransactionFactory(),
		executorFactory:            usecases.NewExecutorFactory(),
		dataModelRepository:        usecases.Repositories.DataModelRepository,
		populateOrganizationSchema: usecases.NewPopulateOrganizationSchema(),
	}
}

func (usecases *UsecasesWithCreds) NewIngestionUseCase() IngestionUseCase {
	var gcsRepository repositories.GcsRepository
	if usecases.Configuration.FakeGcsRepository {
		gcsRepository = &repositories.GcsRepositoryFake{}
	} else {
		gcsRepository = usecases.Repositories.GcsRepository
	}

	return IngestionUseCase{
		enforceSecurity:     usecases.NewEnforceIngestionSecurity(),
		transactionFactory:  usecases.NewTransactionFactory(),
		executorFactory:     usecases.NewExecutorFactory(),
		ingestionRepository: usecases.Repositories.IngestionRepository,
		gcsRepository:       gcsRepository,
		dataModelUseCase:    usecases.NewDataModelUseCase(),
		uploadLogRepository: usecases.Repositories.UploadLogRepository,
		GcsIngestionBucket:  usecases.Configuration.GcsIngestionBucket,
	}
}

func (usecases *UsecasesWithCreds) NewRunScheduledExecution() scheduledexecution.RunScheduledExecution {
	return scheduledexecution.RunScheduledExecution{
		Repository:                     &usecases.Repositories.MarbleDbRepository,
		ExecutorFactory:                usecases.NewExecutorFactory(),
		TransactionFactory:             usecases.NewTransactionFactory(),
		ExportScheduleExecution:        *usecases.NewExportScheduleExecution(),
		ScenarioPublicationsRepository: usecases.Repositories.ScenarioPublicationRepository,
		DataModelRepository:            usecases.Repositories.DataModelRepository,
		IngestedDataReadRepository:     usecases.Repositories.IngestedDataReadRepository,
		EvaluateRuleAstExpression:      usecases.NewEvaluateRuleAstExpression(),
		DecisionRepository:             usecases.Repositories.DecisionRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScheduledExecutionUsecase() ScheduledExecutionUsecase {
	return ScheduledExecutionUsecase{
		enforceSecurity:         usecases.NewEnforceDecisionSecurity(),
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		repository:              &usecases.Repositories.MarbleDbRepository,
		exportScheduleExecution: usecases.NewExportScheduleExecution(),
		organizationIdOfContext: usecases.OrganizationIdOfContext,
	}
}

func (usecases *UsecasesWithCreds) NewUserUseCase() UserUseCase {
	return UserUseCase{
		enforceUserSecurity: usecases.NewEnforceUserSecurity(),
		transactionFactory:  usecases.NewTransactionFactory(),
		userRepository:      usecases.Repositories.UserRepository,
	}
}

func (usecases *UsecasesWithCreds) NewCaseUseCase() CaseUseCase {
	var gcsRepository repositories.GcsRepository
	if usecases.Configuration.FakeGcsRepository {
		gcsRepository = &repositories.GcsRepositoryFake{}
	} else {
		gcsRepository = usecases.Repositories.GcsRepository
	}
	sec := security.EnforceSecurityInboxes{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
	return CaseUseCase{
		enforceSecurity:    usecases.NewEnforceCaseSecurity(),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		repository:         &usecases.Repositories.MarbleDbRepository,
		decisionRepository: usecases.Repositories.DecisionRepository,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity:         sec,
			OrganizationIdOfContext: usecases.OrganizationIdOfContext,
			InboxRepository:         &usecases.Repositories.MarbleDbRepository,
			Credentials:             usecases.Credentials,
			ExecutorFactory:         usecases.NewExecutorFactory(),
		},
		gcsCaseManagerBucket: usecases.Configuration.GcsCaseManagerBucket,
		gcsRepository:        gcsRepository,
	}
}

func (usecases *UsecasesWithCreds) NewInboxUsecase() InboxUsecase {
	sec := security.EnforceSecurityInboxes{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
	executorFactory := usecases.NewExecutorFactory()
	return InboxUsecase{
		enforceSecurity:         sec,
		inboxRepository:         &usecases.Repositories.MarbleDbRepository,
		userRepository:          usecases.Repositories.UserRepository,
		credentials:             usecases.Credentials,
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         executorFactory,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity:         sec,
			OrganizationIdOfContext: usecases.OrganizationIdOfContext,
			InboxRepository:         &usecases.Repositories.MarbleDbRepository,
			Credentials:             usecases.Credentials,
			ExecutorFactory:         executorFactory,
		},
		inboxUsers: inboxes.InboxUsers{
			EnforceSecurity:     sec,
			InboxUserRepository: &usecases.Repositories.MarbleDbRepository,
			Credentials:         usecases.Credentials,
			TransactionFactory:  usecases.NewTransactionFactory(),
			ExecutorFactory:     executorFactory,
			UserRepository:      usecases.Repositories.UserRepository,
		},
	}
}

func (usecases *UsecasesWithCreds) NewTagUseCase() TagUseCase {
	sec := security.EnforceSecurityInboxes{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
	return TagUseCase{
		enforceSecurity:    sec,
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		repository:         &usecases.Repositories.MarbleDbRepository,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity:         sec,
			OrganizationIdOfContext: usecases.OrganizationIdOfContext,
			InboxRepository:         &usecases.Repositories.MarbleDbRepository,
			Credentials:             usecases.Credentials,
			ExecutorFactory:         usecases.NewExecutorFactory(),
		},
	}
}

func (usecases *UsecasesWithCreds) NewApiKeyUseCase() ApiKeyUseCase {
	return ApiKeyUseCase{
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity: &security.EnforceSecurityApiKeyImpl{
			EnforceSecurity: usecases.NewEnforceSecurity(),
			Credentials:     usecases.Credentials,
		},
		apiKeyRepository: &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewAnalyticsUseCase() AnalyticsUseCase {
	return AnalyticsUseCase{
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity: &security.EnforceSecurityAnalyticsImpl{
			EnforceSecurity: usecases.NewEnforceSecurity(),
			Credentials:     usecases.Credentials,
		},
		analyticsRepository: &usecases.Repositories.MarbleAnalyticsRepository,
	}
}
