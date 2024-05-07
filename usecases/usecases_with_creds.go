package usecases

import (
	"context"
	"log/slog"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/indexes"
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
		dataModelRepository:        usecases.Repositories.DataModelRepository,
		repository:                 &usecases.Repositories.MarbleDbRepository,
		evaluateAstExpression:      usecases.NewEvaluateAstExpression(),
		organizationIdOfContext:    usecases.OrganizationIdOfContext,
		caseCreator:                usecases.NewCaseUseCase(),
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
		transactionFactory:        usecases.NewTransactionFactory(),
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
		clientDbIndexEditor:            usecases.NewClientDbIndexEditor(),
	}
}

func (usecases *UsecasesWithCreds) NewClientDbIndexEditor() clientDbIndexEditor {
	return indexes.NewClientDbIndexEditor(
		usecases.NewExecutorFactory(),
		usecases.NewScenarioFetcher(),
		&usecases.Repositories.ClientDbRepository,
		usecases.NewEnforceScenarioSecurity(),
		usecases.NewEnforceOrganizationSecurity(),
		usecases.OrganizationIdOfContext,
	)
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
	}
}

func (usecases *UsecasesWithCreds) NewDataModelUseCase() DataModelUseCase {
	return DataModelUseCase{
		clientDbIndexEditor:          usecases.NewClientDbIndexEditor(),
		dataModelRepository:          usecases.Repositories.DataModelRepository,
		enforceSecurity:              usecases.NewEnforceOrganizationSecurity(),
		executorFactory:              usecases.NewExecutorFactory(),
		organizationSchemaRepository: usecases.Repositories.OrganizationSchemaRepository,
		transactionFactory:           usecases.NewTransactionFactory(),
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
		dataModelRepository: usecases.Repositories.DataModelRepository,
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
		EvaluateAstExpression:          usecases.NewEvaluateAstExpression(),
		DecisionRepository:             usecases.Repositories.DecisionRepository,
		CaseCreator:                    usecases.NewCaseUseCase(),
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

func (usecases *UsecasesWithCreds) NewCaseUseCase() *CaseUseCase {
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
	return &CaseUseCase{
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

func (usecases *UsecasesWithCreds) NewTransferCheckUsecase() TransferCheckUsecase {
	return TransferCheckUsecase{
		dataModelRepository:               usecases.Repositories.DataModelRepository,
		decisionUseCase:                   usecases.NewDecisionUsecase(),
		decisionRepository:                usecases.Repositories.DecisionRepository,
		enforceSecurity:                   security.NewEnforceSecurity(usecases.Credentials),
		executorFactory:                   usecases.NewExecutorFactory(),
		ingestedDataReadRepository:        usecases.Repositories.IngestedDataReadRepository,
		ingestionRepository:               usecases.Repositories.IngestionRepository,
		organizationRepository:            usecases.Repositories.OrganizationRepository,
		transactionFactory:                usecases.NewTransactionFactory(),
		transferMappingsRepository:        &usecases.Repositories.MarbleDbRepository,
		transferCheckEnrichmentRepository: repositories.NewTransferCheckEnrichmentRepository(),
	}
}

func (usecases *UsecasesWithCreds) NewPartnerUsecase() PartnerUsecase {
	return PartnerUsecase{
		enforceSecurity:    security.NewEnforceSecurity(usecases.Credentials),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		partnersRepository: usecases.Repositories.MarbleDbRepository,
	}
}
