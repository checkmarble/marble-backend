package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/checkmarble/marble-backend/usecases/org_transaction"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/scheduledexecution"
	"github.com/checkmarble/marble-backend/usecases/security"

	"slices"
)

type Usecases struct {
	Repositories  repositories.Repositories
	Configuration models.GlobalConfiguration
}

func (usecases *Usecases) NewOrgTransactionFactory() org_transaction.Factory {
	return &org_transaction.FactoryImpl{
		OrganizationSchemaRepository:     usecases.Repositories.OrganizationSchemaRepository,
		TransactionFactory:               usecases.Repositories.TransactionFactory,
		DatabaseConnectionPoolRepository: usecases.Repositories.DatabaseConnectionPoolRepository,
	}
}

func (usecases *Usecases) NewSeedUseCase() SeedUseCase {
	return SeedUseCase{
		transactionFactory:     usecases.Repositories.TransactionFactory,
		userRepository:         usecases.Repositories.UserRepository,
		organizationCreator:    usecases.NewOrganizationCreator(),
		organizationRepository: usecases.Repositories.OrganizationRepository,
		customListRepository:   usecases.Repositories.CustomListRepository,
	}
}

func (usecases *Usecases) NewOrganizationCreator() organization.OrganizationCreator {
	return organization.OrganizationCreator{
		TransactionFactory:     usecases.Repositories.TransactionFactory,
		OrganizationRepository: usecases.Repositories.OrganizationRepository,
		DataModelRepository:    usecases.Repositories.DataModelRepository,
		OrganizationSeeder: organization.OrganizationSeeder{
			CustomListRepository:             usecases.Repositories.CustomListRepository,
			ApiKeyRepository:                 usecases.Repositories.ApiKeyRepository,
			ScenarioWriteRepository:          usecases.Repositories.ScenarioWriteRepository,
			ScenarioIterationWriteRepository: usecases.Repositories.ScenarioIterationWriteRepository,
			ScenarioPublisher:                usecases.NewScenarioPublisher(),
		},
		PopulateOrganizationSchema: usecases.NewPopulateOrganizationSchema(),
	}
}

func (usecases *Usecases) NewExportScheduleExecution() scheduledexecution.ExportScheduleExecution {

	var awsS3Repository scheduledexecution.AwsS3Repository
	if usecases.Configuration.FakeAwsS3Repository {
		awsS3Repository = &repositories.AwsS3RepositoryFake{}
	} else {
		awsS3Repository = &usecases.Repositories.AwsS3Repository
	}

	return scheduledexecution.ExportScheduleExecution{
		AwsS3Repository:        awsS3Repository,
		DecisionRepository:     usecases.Repositories.DecisionRepository,
		OrganizationRepository: usecases.Repositories.OrganizationRepository,
	}
}

func (usecases *Usecases) NewPopulateOrganizationSchema() organization.PopulateOrganizationSchema {
	return organization.PopulateOrganizationSchema{
		TransactionFactory:           usecases.Repositories.TransactionFactory,
		OrganizationRepository:       usecases.Repositories.OrganizationRepository,
		OrganizationSchemaRepository: usecases.Repositories.OrganizationSchemaRepository,
		DataModelRepository:          usecases.Repositories.DataModelRepository,
	}
}

func (usecases *Usecases) AstEvaluationEnvironmentFactory(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
	environment := ast_eval.NewAstEvaluationEnvironment()

	// execution of a scenario with a dedicated security context
	enforceSecurity := &security.EnforceSecurityImpl{
		Credentials: models.Credentials{
			OrganizationId: params.OrganizationId,
		},
	}

	environment.AddEvaluator(ast.FUNC_CUSTOM_LIST_ACCESS,
		evaluate.NewCustomListValuesAccess(
			usecases.Repositories.CustomListRepository,
			enforceSecurity,
		),
	)

	environment.AddEvaluator(ast.FUNC_DB_ACCESS,
		evaluate.DatabaseAccess{
			OrganizationId:             params.OrganizationId,
			DataModel:                  params.DataModel,
			Payload:                    params.Payload,
			OrgTransactionFactory:      usecases.NewOrgTransactionFactory(),
			IngestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
			ReturnFakeValue:            params.DatabaseAccessReturnFakeValue,
		},
	)

	environment.AddEvaluator(ast.FUNC_PAYLOAD, evaluate.NewPayload(ast.FUNC_PAYLOAD, params.Payload))

	environment.AddEvaluator(ast.FUNC_AGGREGATOR, evaluate.AggregatorEvaluator{
		OrganizationId:             params.OrganizationId,
		DataModel:                  params.DataModel,
		Payload:                    params.Payload,
		OrgTransactionFactory:      usecases.NewOrgTransactionFactory(),
		IngestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
		ReturnFakeValue:            params.DatabaseAccessReturnFakeValue,
	})

	environment.AddEvaluator(ast.FUNC_FILTER, evaluate.FilterEvaluator{
		DataModel: params.DataModel,
	})

	// Custom evaluators for the Blank organization
	if slices.Contains(models.GetBlankOrganizationIds(), params.OrganizationId) {
		addBlankVariableEvaluators(&environment, usecases, params.OrganizationId, params.DatabaseAccessReturnFakeValue)
	}
	return environment
}

func (usecases *Usecases) NewEvaluateRuleAstExpression() ast_eval.EvaluateRuleAstExpression {
	return ast_eval.EvaluateRuleAstExpression{
		AstEvaluationEnvironmentFactory: usecases.AstEvaluationEnvironmentFactory,
	}
}

func (usecases *Usecases) NewScenarioPublisher() scenarios.ScenarioPublisher {
	return scenarios.ScenarioPublisher{
		ScenarioPublicationsRepository:   usecases.Repositories.ScenarioPublicationRepository,
		ScenarioWriteRepository:          usecases.Repositories.ScenarioWriteRepository,
		ScenarioIterationReadRepository:  usecases.Repositories.ScenarioIterationReadRepository,
		ScenarioIterationWriteRepository: usecases.Repositories.ScenarioIterationWriteRepository,
		ValidateScenarioIteration:        usecases.NewValidateScenarioIteration(),
	}
}

func (usecases *Usecases) NewValidateScenarioIteration() scenarios.ValidateScenarioIteration {
	return &scenarios.ValidateScenarioIterationImpl{
		DataModelRepository:             usecases.Repositories.DataModelRepository,
		AstEvaluationEnvironmentFactory: usecases.AstEvaluationEnvironmentFactory,
	}
}

func (usecase *Usecases) NewScenarioFetcher() scenarios.ScenarioFetcher {
	return scenarios.ScenarioFetcher{
		ScenarioReadRepository:          usecase.Repositories.ScenarioReadRepository,
		ScenarioIterationReadRepository: usecase.Repositories.ScenarioIterationReadRepository,
	}
}
