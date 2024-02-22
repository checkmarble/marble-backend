package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/scheduledexecution"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type Usecases struct {
	Repositories  repositories.Repositories
	Configuration models.GlobalConfiguration
}

func (usecases *Usecases) NewExecutorFactory() executor_factory.ExecutorFactory {
	return executor_factory.NewDbExecutorFactory(
		usecases.Repositories.OrganizationSchemaRepository,
		usecases.Repositories.ExecutorGetter,
	)
}

func (usecases *Usecases) NewTransactionFactory() executor_factory.TransactionFactory {
	return executor_factory.NewDbExecutorFactory(
		usecases.Repositories.OrganizationSchemaRepository,
		usecases.Repositories.ExecutorGetter,
	)
}

func (usecases *Usecases) NewSeedUseCase() SeedUseCase {
	return SeedUseCase{
		transactionFactory:     usecases.NewTransactionFactory(),
		executorFactory:        usecases.NewExecutorFactory(),
		userRepository:         usecases.Repositories.UserRepository,
		organizationCreator:    usecases.NewOrganizationCreator(),
		organizationRepository: usecases.Repositories.OrganizationRepository,
		customListRepository:   usecases.Repositories.CustomListRepository,
	}
}

func (usecases *Usecases) NewOrganizationCreator() organization.OrganizationCreator {
	return organization.OrganizationCreator{
		TransactionFactory:     usecases.NewTransactionFactory(),
		OrganizationRepository: usecases.Repositories.OrganizationRepository,
		DataModelRepository:    usecases.Repositories.DataModelRepository,
		OrganizationSeeder: organization.OrganizationSeeder{
			ExecutorFactory:      usecases.NewExecutorFactory(),
			CustomListRepository: usecases.Repositories.CustomListRepository,
		},
	}
}

func (usecases *Usecases) NewExportScheduleExecution() *scheduledexecution.ExportScheduleExecution {
	var awsS3Repository scheduledexecution.AwsS3Repository
	if usecases.Configuration.FakeAwsS3Repository {
		awsS3Repository = &repositories.AwsS3RepositoryFake{}
	} else {
		awsS3Repository = &usecases.Repositories.AwsS3Repository
	}

	return &scheduledexecution.ExportScheduleExecution{
		AwsS3Repository:        awsS3Repository,
		DecisionRepository:     usecases.Repositories.DecisionRepository,
		OrganizationRepository: usecases.Repositories.OrganizationRepository,
		ExecutorFactory:        usecases.NewExecutorFactory(),
	}
}

func (usecases *Usecases) NewPopulateOrganizationSchema() organization.PopulateOrganizationSchema {
	return organization.PopulateOrganizationSchema{
		ExecutorFactory:              usecases.NewExecutorFactory(),
		OrganizationSchemaRepository: usecases.Repositories.OrganizationSchemaRepository,
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
			usecases.NewExecutorFactory(),
		),
	)

	environment.AddEvaluator(ast.FUNC_DB_ACCESS,
		evaluate.DatabaseAccess{
			OrganizationId:             params.OrganizationId,
			DataModel:                  params.DataModel,
			ClientObject:               params.ClientObject,
			ExecutorFactory:            usecases.NewExecutorFactory(),
			IngestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
			ReturnFakeValue:            params.DatabaseAccessReturnFakeValue,
		},
	)

	environment.AddEvaluator(ast.FUNC_PAYLOAD,
		evaluate.NewPayload(ast.FUNC_PAYLOAD, params.ClientObject))

	environment.AddEvaluator(ast.FUNC_AGGREGATOR, evaluate.AggregatorEvaluator{
		OrganizationId:             params.OrganizationId,
		DataModel:                  params.DataModel,
		ClientObject:               params.ClientObject,
		ExecutorFactory:            usecases.NewExecutorFactory(),
		IngestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
		ReturnFakeValue:            params.DatabaseAccessReturnFakeValue,
	})

	environment.AddEvaluator(ast.FUNC_FILTER, evaluate.FilterEvaluator{
		DataModel: params.DataModel,
	})

	return environment
}

func (usecases *Usecases) NewEvaluateRuleAstExpression() ast_eval.EvaluateRuleAstExpression {
	return ast_eval.EvaluateRuleAstExpression{
		AstEvaluationEnvironmentFactory: usecases.AstEvaluationEnvironmentFactory,
	}
}

func (usecases *Usecases) NewScenarioPublisher() ScenarioPublisher {
	return scenarios.ScenarioPublisher{
		Repository:                     &usecases.Repositories.MarbleDbRepository,
		ValidateScenarioIteration:      usecases.NewValidateScenarioIteration(),
		ScenarioPublicationsRepository: usecases.Repositories.ScenarioPublicationRepository,
	}
}

func (usecases *Usecases) NewValidateScenarioIteration() scenarios.ValidateScenarioIteration {
	return &scenarios.ValidateScenarioIterationImpl{
		DataModelRepository:             usecases.Repositories.DataModelRepository,
		AstEvaluationEnvironmentFactory: usecases.AstEvaluationEnvironmentFactory,
		ExecutorFactory:                 usecases.NewExecutorFactory(),
	}
}

func (usecase *Usecases) NewScenarioFetcher() scenarios.ScenarioFetcher {
	return scenarios.ScenarioFetcher{
		Repository: &usecase.Repositories.MarbleDbRepository,
	}
}
