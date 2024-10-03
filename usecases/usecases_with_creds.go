package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/decision_workflows"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/usecases/scheduled_execution"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transfers_data_read"
)

type UsecasesWithCreds struct {
	Usecases
	Credentials models.Credentials
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

func (usecases *UsecasesWithCreds) NewEnforceTagSecurity() security.EnforceSecurityTags {
	return &security.EnforceSecurityImpl{
		Credentials: usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewDecisionUsecase() DecisionUsecase {
	return DecisionUsecase{
		enforceSecurity:            usecases.NewEnforceDecisionSecurity(),
		enforceSecurityScenario:    usecases.NewEnforceScenarioSecurity(),
		executorFactory:            usecases.NewExecutorFactory(),
		transactionFactory:         usecases.NewTransactionFactory(),
		ingestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
		dataModelRepository:        usecases.Repositories.DataModelRepository,
		repository:                 &usecases.Repositories.MarbleDbRepository,
		evaluateAstExpression:      usecases.NewEvaluateAstExpression(),
		decisionWorkflows:          usecases.NewDecisionWorkflows(),
		webhookEventsSender:        usecases.NewWebhookEventsUsecase(),
		snoozesReader:              &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewDecisionWorkflows() decision_workflows.DecisionsWorkflows {
	return decision_workflows.NewDecisionWorkflows(
		usecases.NewCaseUseCase(),
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewWebhookEventsUsecase(),
	)
}

func (usecases *UsecasesWithCreds) NewScenarioUsecase() ScenarioUsecase {
	return ScenarioUsecase{
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		enforceSecurity:    usecases.NewEnforceScenarioSecurity(),
		repository:         &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioIterationUsecase() ScenarioIterationUsecase {
	return ScenarioIterationUsecase{
		repository:                &usecases.Repositories.MarbleDbRepository,
		enforceSecurity:           usecases.NewEnforceScenarioSecurity(),
		scenarioFetcher:           usecases.NewScenarioFetcher(),
		validateScenarioIteration: usecases.NewValidateScenarioIteration(),
		executorFactory:           usecases.NewExecutorFactory(),
		transactionFactory:        usecases.NewTransactionFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewRuleUsecase() RuleUsecase {
	return RuleUsecase{
		enforceSecurity:    usecases.NewEnforceScenarioSecurity(),
		repository:         &usecases.Repositories.MarbleDbRepository,
		scenarioFetcher:    usecases.NewScenarioFetcher(),
		transactionFactory: usecases.NewTransactionFactory(),
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
		enforceSecurity:      usecases.NewEnforceCustomListSecurity(),
		transactionFactory:   usecases.NewTransactionFactory(),
		executorFactory:      usecases.NewExecutorFactory(),
		CustomListRepository: usecases.Repositories.CustomListRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioPublicationUsecase() ScenarioPublicationUsecase {
	return ScenarioPublicationUsecase{
		transactionFactory:             usecases.NewTransactionFactory(),
		executorFactory:                usecases.NewExecutorFactory(),
		scenarioPublicationsRepository: usecases.Repositories.ScenarioPublicationRepository,
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
	return IngestionUseCase{
		enforceSecurity:     usecases.NewEnforceIngestionSecurity(),
		transactionFactory:  usecases.NewTransactionFactory(),
		executorFactory:     usecases.NewExecutorFactory(),
		ingestionRepository: usecases.Repositories.IngestionRepository,
		blobRepository:      usecases.Repositories.BlobRepository,
		dataModelRepository: usecases.Repositories.DataModelRepository,
		uploadLogRepository: usecases.Repositories.UploadLogRepository,
		GcsIngestionBucket:  usecases.gcsIngestionBucket,
	}
}

func (usecases *UsecasesWithCreds) NewRunScheduledExecution() scheduled_execution.RunScheduledExecution {
	return *scheduled_execution.NewRunScheduledExecution(
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.Repositories.ScenarioPublicationRepository,
		usecases.Repositories.DataModelRepository,
		usecases.Repositories.IngestedDataReadRepository,
		usecases.NewEvaluateAstExpression(),
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewTransactionFactory(),
		usecases.NewDecisionWorkflows(),
		usecases.NewWebhookEventsUsecase(),
		&usecases.Repositories.MarbleDbRepository,
	)
}

func (usecases *UsecasesWithCreds) NewScheduledExecutionUsecase() ScheduledExecutionUsecase {
	return ScheduledExecutionUsecase{
		enforceSecurity:         usecases.NewEnforceDecisionSecurity(),
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		repository:              &usecases.Repositories.MarbleDbRepository,
		exportScheduleExecution: usecases.NewExportScheduleExecution(),
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
	sec := security.EnforceSecurityInboxes{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
	return &CaseUseCase{
		enforceSecurity:    usecases.NewEnforceCaseSecurity(),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		repository:         &usecases.Repositories.MarbleDbRepository,
		decisionRepository: &usecases.Repositories.MarbleDbRepository,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity: sec,
			InboxRepository: &usecases.Repositories.MarbleDbRepository,
			Credentials:     usecases.Credentials,
			ExecutorFactory: usecases.NewExecutorFactory(),
		},
		gcsCaseManagerBucket: usecases.gcsCaseManagerBucket,
		blobRepository:       usecases.Repositories.BlobRepository,
		webhookEventsUsecase: usecases.NewWebhookEventsUsecase(),
	}
}

func (usecases *UsecasesWithCreds) NewInboxUsecase() InboxUsecase {
	sec := security.EnforceSecurityInboxes{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
	executorFactory := usecases.NewExecutorFactory()
	return InboxUsecase{
		enforceSecurity:    sec,
		inboxRepository:    &usecases.Repositories.MarbleDbRepository,
		userRepository:     usecases.Repositories.UserRepository,
		credentials:        usecases.Credentials,
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    executorFactory,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity: sec,
			InboxRepository: &usecases.Repositories.MarbleDbRepository,
			Credentials:     usecases.Credentials,
			ExecutorFactory: executorFactory,
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
	return TagUseCase{
		enforceSecurity:    usecases.NewEnforceTagSecurity(),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		repository:         &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewApiKeyUseCase() ApiKeyUseCase {
	return ApiKeyUseCase{
		executorFactory: usecases.NewExecutorFactory(),
		enforceSecurity: &security.EnforceSecurityApiKeyImpl{
			EnforceSecurity: usecases.NewEnforceSecurity(),
			Credentials:     usecases.Credentials,
		},
		apiKeyRepository: &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewAnalyticsUseCase() AnalyticsUseCase {
	return AnalyticsUseCase{
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
		decisionRepository:                &usecases.Repositories.MarbleDbRepository,
		enforceSecurity:                   security.NewEnforceSecurity(usecases.Credentials),
		executorFactory:                   usecases.NewExecutorFactory(),
		ingestionRepository:               usecases.Repositories.IngestionRepository,
		organizationRepository:            usecases.Repositories.OrganizationRepository,
		transactionFactory:                usecases.NewTransactionFactory(),
		transferMappingsRepository:        &usecases.Repositories.MarbleDbRepository,
		transferCheckEnrichmentRepository: usecases.Repositories.TransferCheckEnrichmentRepository,
		transferDataReader:                usecases.NewTransferDataReader(),
		partnersRepository:                usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewTransferAlertsUsecase() TransferAlertsUsecase {
	return NewTransferAlertsUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.Repositories.OrganizationRepository,
		usecases.NewTransactionFactory(),
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewTransferDataReader(),
	)
}

func (usecases *UsecasesWithCreds) NewTransferDataReader() transfers_data_read.TransferDataReader {
	return transfers_data_read.NewTransferDataReader(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.Repositories.IngestedDataReadRepository,
		usecases.Repositories.DataModelRepository,
	)
}

func (usecases *UsecasesWithCreds) NewPartnerUsecase() PartnerUsecase {
	return PartnerUsecase{
		enforceSecurity:    security.NewEnforceSecurity(usecases.Credentials),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		partnersRepository: usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewLicenseUsecase() ProtectedLicenseUseCase {
	return ProtectedLicenseUseCase{
		enforceSecurity:    security.NewEnforceSecurity(usecases.Credentials),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		licenseRepository:  &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewWebhookEventsUsecase() WebhookEventsUsecase {
	return NewWebhookEventsUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.Repositories.ConvoyRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Usecases.failedWebhooksRetryPageSize,
		usecases.Usecases.license.Webhooks,
	)
}

func (usecases *UsecasesWithCreds) NewWebhooksUsecase() WebhooksUsecase {
	return NewWebhooksUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.ConvoyRepository,
	)
}

func (usecases *UsecasesWithCreds) NewRuleSnoozeUsecase() RuleSnoozeUsecase {
	return NewRuleSnoozeUsecase(
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.NewCaseUseCase(),
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewWebhookEventsUsecase(),
	)
}
