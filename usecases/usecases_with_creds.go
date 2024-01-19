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
		transactionFactory:         &usecases.Repositories.TransactionFactoryPosgresql,
		orgTransactionFactory:      usecases.NewOrgTransactionFactory(),
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
		transactionFactory:      &usecases.Repositories.TransactionFactoryPosgresql,
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
	}
}

func (usecases *UsecasesWithCreds) NewRuleUsecase() RuleUsecase {
	return RuleUsecase{
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		enforceSecurity:         usecases.NewEnforceScenarioSecurity(),
		repository:              &usecases.Repositories.MarbleDbRepository,
		scenarioFetcher:         usecases.NewScenarioFetcher(),
		transactionFactory:      &usecases.Repositories.TransactionFactoryPosgresql,
	}
}

func (usecases *UsecasesWithCreds) AstExpressionUsecase() AstExpressionUsecase {
	return AstExpressionUsecase{
		EnforceSecurity:     usecases.NewEnforceScenarioSecurity(),
		DataModelRepository: usecases.Repositories.DataModelRepository,
		Repository:          &usecases.Repositories.MarbleDbRepository,
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

func (usecases *UsecasesWithCreds) NewDataModelUseCase() DataModelUseCase {
	return DataModelUseCase{
		enforceSecurity:            usecases.NewEnforceOrganizationSecurity(),
		transactionFactory:         &usecases.Repositories.TransactionFactoryPosgresql,
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
		enforceSecurity:       usecases.NewEnforceIngestionSecurity(),
		transactionFactory:    &usecases.Repositories.TransactionFactoryPosgresql,
		orgTransactionFactory: usecases.NewOrgTransactionFactory(),
		ingestionRepository:   usecases.Repositories.IngestionRepository,
		gcsRepository:         gcsRepository,
		dataModelUseCase:      usecases.NewDataModelUseCase(),
		uploadLogRepository:   usecases.Repositories.UploadLogRepository,
		GcsIngestionBucket:    usecases.Configuration.GcsIngestionBucket,
	}

}

func (usecases *UsecasesWithCreds) NewRunScheduledExecution() scheduledexecution.RunScheduledExecution {
	return scheduledexecution.RunScheduledExecution{
		Repository:                     &usecases.Repositories.MarbleDbRepository,
		TransactionFactory:             &usecases.Repositories.TransactionFactoryPosgresql,
		ExportScheduleExecution:        *usecases.NewExportScheduleExecution(),
		ScenarioPublicationsRepository: usecases.Repositories.ScenarioPublicationRepository,
		DataModelRepository:            usecases.Repositories.DataModelRepository,
		OrgTransactionFactory:          usecases.NewOrgTransactionFactory(),
		IngestedDataReadRepository:     usecases.Repositories.IngestedDataReadRepository,
		EvaluateRuleAstExpression:      usecases.NewEvaluateRuleAstExpression(),
		DecisionRepository:             usecases.Repositories.DecisionRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScheduledExecutionUsecase() ScheduledExecutionUsecase {
	return ScheduledExecutionUsecase{
		enforceSecurity:         usecases.NewEnforceDecisionSecurity(),
		transactionFactory:      &usecases.Repositories.TransactionFactoryPosgresql,
		repository:              &usecases.Repositories.MarbleDbRepository,
		exportScheduleExecution: usecases.NewExportScheduleExecution(),
		organizationIdOfContext: usecases.OrganizationIdOfContext,
	}
}

func (usecases *UsecasesWithCreds) NewUserUseCase() UserUseCase {
	return UserUseCase{
		enforceUserSecurity: usecases.NewEnforceUserSecurity(),
		transactionFactory:  &usecases.Repositories.TransactionFactoryPosgresql,
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
		transactionFactory: &usecases.Repositories.TransactionFactoryPosgresql,
		repository:         &usecases.Repositories.MarbleDbRepository,
		decisionRepository: usecases.Repositories.DecisionRepository,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity:         sec,
			OrganizationIdOfContext: usecases.OrganizationIdOfContext,
			InboxRepository:         &usecases.Repositories.MarbleDbRepository,
			Credentials:             usecases.Credentials,
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
	return InboxUsecase{
		enforceSecurity:         sec,
		inboxRepository:         &usecases.Repositories.MarbleDbRepository,
		userRepository:          usecases.Repositories.UserRepository,
		credentials:             usecases.Credentials,
		organizationIdOfContext: usecases.OrganizationIdOfContext,
		transactionFactory:      &usecases.Repositories.TransactionFactoryPosgresql,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity:         sec,
			OrganizationIdOfContext: usecases.OrganizationIdOfContext,
			InboxRepository:         &usecases.Repositories.MarbleDbRepository,
			Credentials:             usecases.Credentials,
		},
		inboxUsers: inboxes.InboxUsers{
			EnforceSecurity:     sec,
			InboxUserRepository: &usecases.Repositories.MarbleDbRepository,
			Credentials:         usecases.Credentials,
			TransactionFactory:  &usecases.Repositories.TransactionFactoryPosgresql,
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
		transactionFactory: &usecases.Repositories.TransactionFactoryPosgresql,
		repository:         &usecases.Repositories.MarbleDbRepository,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity:         sec,
			OrganizationIdOfContext: usecases.OrganizationIdOfContext,
			InboxRepository:         &usecases.Repositories.MarbleDbRepository,
			Credentials:             usecases.Credentials,
		},
	}
}
