package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

type ScenarioPublicationUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity                *mocks.EnforceSecurity
	executorFactory                *mocks.ExecutorFactory
	scenarioFetcher                *mocks.ScenarioFetcher
	scenarioPublicationsRepository *mocks.ScenarioPublicationRepository
	scenarioPublisher              *mocks.ScenarioPublisher
	transaction                    *mocks.Executor
	transactionFactory             *mocks.TransactionFactory
	clientDbIndexEditor            *mocks.ClientDbIndexEditor

	organizationId                string
	scenarioId                    string
	iterationId                   string
	publicationId                 string
	scenarioPublication           models.ScenarioPublication
	scenario                      models.Scenario
	scenarioIteration             models.ScenarioIteration
	scenarioAndIteration          models.ScenarioAndIteration
	scenarioAndIterationWithQuery models.ScenarioAndIteration
	existingIndexes               []models.ConcreteIndex

	repositoryError error
	securityError   error
	ctx             context.Context
}

func (suite *ScenarioPublicationUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.scenarioFetcher = new(mocks.ScenarioFetcher)
	suite.scenarioPublicationsRepository = new(mocks.ScenarioPublicationRepository)
	suite.scenarioPublisher = new(mocks.ScenarioPublisher)
	suite.transaction = new(mocks.Executor)
	suite.transactionFactory = &mocks.TransactionFactory{ExecMock: suite.transaction}
	suite.clientDbIndexEditor = new(mocks.ClientDbIndexEditor)

	suite.organizationId = "organizationId"
	suite.scenarioId = "scenarioId"
	suite.iterationId = "iterationId"
	suite.publicationId = "publicationId"
	suite.scenarioPublication = models.ScenarioPublication{
		Id:                  suite.publicationId,
		OrganizationId:      suite.organizationId,
		ScenarioId:          suite.scenarioId,
		ScenarioIterationId: suite.iterationId,
		PublicationAction:   models.Publish,
	}
	suite.scenario = models.Scenario{
		Id:             suite.scenarioId,
		OrganizationId: suite.organizationId,
	}
	suite.scenarioIteration = models.ScenarioIteration{
		Id:                            suite.iterationId,
		OrganizationId:                suite.organizationId,
		ScenarioId:                    suite.scenarioId,
		TriggerConditionAstExpression: &ast.Node{},
	}
	suite.scenarioAndIteration = models.ScenarioAndIteration{
		Scenario:  suite.scenario,
		Iteration: suite.scenarioIteration,
	}

	// setup an iteration that requires an index to be created
	suite.scenarioAndIterationWithQuery = models.ScenarioAndIteration{
		Scenario:  suite.scenario,
		Iteration: suite.scenarioIteration,
	}
	astJson := `{
		"name": "Or",
		"children": [
		  {
		    "name": "And",
		    "children": [
			{
			  "name": "\u003e",
			  "children": [
			    {
				"name": "Aggregator",
				"named_children": {
				  "aggregator": { "constant": "COUNT_DISTINCT" },
				  "fieldName": { "constant": "object_id" },
				  "filters": {
				    "name": "List",
				    "children": [
					{
					  "name": "Filter",
					  "named_children": {
					    "fieldName": { "constant": "new_field" },
					    "operator": { "constant": "=" },
					    "tableName": { "constant": "table" },
					    "value": { "constant": "dummy" }
					  }
					}
				    ]
				  },
				  "label": { "constant": "test" },
				  "tableName": { "constant": "table" }
				}
			    },
			    { "constant": 0 }
			  ]
			}
		    ]
		  }
		]
	    }`
	astNodeDto := dto.NodeDto{}
	err := json.Unmarshal([]byte(astJson), &astNodeDto)
	suite.Require().NoError(err)
	astNode, err := dto.AdaptASTNode(astNodeDto)
	suite.Require().NoError(err)
	suite.scenarioAndIterationWithQuery.Iteration.TriggerConditionAstExpression = &astNode
	suite.existingIndexes = []models.ConcreteIndex{
		{
			TableName: "table", Indexed: []string{"a", "b"},
			Included: []string{"c", "d"},
		},
	}

	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
	suite.ctx = utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
}

func (suite *ScenarioPublicationUsecaseTestSuite) makeUsecase() *ScenarioPublicationUsecase {
	return &ScenarioPublicationUsecase{
		enforceSecurity:                suite.enforceSecurity,
		executorFactory:                suite.executorFactory,
		scenarioFetcher:                suite.scenarioFetcher,
		scenarioPublicationsRepository: suite.scenarioPublicationsRepository,
		scenarioPublisher:              suite.scenarioPublisher,
		transactionFactory:             suite.transactionFactory,
		clientDbIndexEditor:            suite.clientDbIndexEditor,
	}
}

func (suite *ScenarioPublicationUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.executorFactory.AssertExpectations(t)
	suite.scenarioFetcher.AssertExpectations(t)
	suite.scenarioPublicationsRepository.AssertExpectations(t)
	suite.scenarioPublisher.AssertExpectations(t)
	suite.transaction.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
}

// GetScenarioPublication
func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetScenarioPublication_nominal() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.enforceSecurity.On("ReadScenarioPublication", suite.scenarioPublication).Return(nil)
	suite.scenarioPublicationsRepository.On(
		"GetScenarioPublicationById",
		suite.ctx,
		suite.transaction,
		suite.publicationId,
	).Return(suite.scenarioPublication, nil)

	publication, err := suite.makeUsecase().GetScenarioPublication(suite.ctx, suite.publicationId)

	suite.NoError(err)
	suite.Assert().NotEmpty(publication.Id)
	suite.Equal(suite.scenarioPublication, publication)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetScenarioPublication_get_error() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.scenarioPublicationsRepository.On(
		"GetScenarioPublicationById",
		suite.ctx,
		suite.transaction,
		suite.publicationId,
	).Return(models.ScenarioPublication{}, suite.repositoryError)

	publication, err := suite.makeUsecase().GetScenarioPublication(suite.ctx, suite.publicationId)

	suite.Equal(suite.repositoryError, err)
	suite.Assert().Empty(publication.Id)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetScenarioPublication_security_error() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.scenarioPublicationsRepository.On(
		"GetScenarioPublicationById",
		suite.ctx,
		suite.transaction,
		suite.publicationId,
	).Return(suite.scenarioPublication, nil)
	suite.enforceSecurity.On("ReadScenarioPublication", suite.scenarioPublication).Return(suite.securityError)

	publication, err := suite.makeUsecase().GetScenarioPublication(suite.ctx, suite.publicationId)

	suite.Equal(suite.securityError, err)
	suite.Assert().Empty(publication.Id)

	suite.AssertExpectations()
}

// ListScenarioPublications
func (suite *ScenarioPublicationUsecaseTestSuite) Test_ListScenarioPublications_nominal() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.enforceSecurity.On("ListScenarios", suite.organizationId).Return(nil)
	suite.scenarioPublicationsRepository.On(
		"ListScenarioPublicationsOfOrganization",
		suite.ctx,
		suite.transaction,
		suite.organizationId,
		models.ListScenarioPublicationsFilters{},
	).Return([]models.ScenarioPublication{suite.scenarioPublication}, nil)

	publications, err := suite.makeUsecase().ListScenarioPublications(
		suite.ctx,
		suite.organizationId,
		models.ListScenarioPublicationsFilters{})

	suite.NoError(err)
	suite.Assert().NotEmpty(publications)
	suite.Equal(suite.scenarioPublication, publications[0])

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_ListScenarioPublications_security_error() {
	suite.enforceSecurity.On("ListScenarios", suite.organizationId).Return(suite.securityError)

	publications, err := suite.makeUsecase().ListScenarioPublications(
		suite.ctx,
		suite.organizationId,
		models.ListScenarioPublicationsFilters{})

	suite.Equal(suite.securityError, err)
	suite.Assert().Empty(publications)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_ListScenarioPublications_get_error() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.enforceSecurity.On("ListScenarios", suite.organizationId).Return(nil)
	suite.scenarioPublicationsRepository.On(
		"ListScenarioPublicationsOfOrganization",
		suite.ctx,
		suite.transaction,
		suite.organizationId,
		models.ListScenarioPublicationsFilters{},
	).Return([]models.ScenarioPublication{}, suite.repositoryError)

	publications, err := suite.makeUsecase().ListScenarioPublications(
		suite.ctx,
		suite.organizationId,
		models.ListScenarioPublicationsFilters{})

	suite.Equal(suite.repositoryError, err)
	suite.Assert().Empty(publications)

	suite.AssertExpectations()
}

// ExecuteScenarioPublicationAction
func (suite *ScenarioPublicationUsecaseTestSuite) Test_ExecuteScenarioPublicationAction_nominal() {
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId, suite.iterationId).Return(
		[]models.ConcreteIndex{}, 0, nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.scenarioPublisher.On("PublishOrUnpublishIteration", suite.ctx, suite.transaction, mock.Anything, models.Publish).
		Return([]models.ScenarioPublication{suite.scenarioPublication}, nil)

	publications, err := suite.makeUsecase().ExecuteScenarioPublicationAction(suite.ctx,
		suite.organizationId,
		models.PublishScenarioIterationInput{
			ScenarioIterationId: suite.iterationId,
			PublicationAction:   models.Publish,
		})

	suite.NoError(err)
	suite.Assert().NotEmpty(publications)
	suite.Equal(suite.scenarioPublication, publications[0])

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_ExecuteScenarioPublicationAction_fetch_error() {
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId, suite.iterationId).Return(
		suite.existingIndexes, 0, nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(models.ScenarioAndIteration{}, suite.repositoryError)

	publications, err := suite.makeUsecase().ExecuteScenarioPublicationAction(
		suite.ctx,
		suite.organizationId,
		models.PublishScenarioIterationInput{
			ScenarioIterationId: suite.iterationId,
			PublicationAction:   models.Unpublish,
		})

	suite.Equal(suite.repositoryError, err)
	suite.Assert().Empty(publications)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_ExecuteScenarioPublicationAction_security_error() {
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId, suite.iterationId).Return(
		[]models.ConcreteIndex{}, 0, nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(suite.securityError)

	publications, err := suite.makeUsecase().ExecuteScenarioPublicationAction(
		suite.ctx,
		suite.organizationId,
		models.PublishScenarioIterationInput{
			ScenarioIterationId: suite.iterationId,
			PublicationAction:   models.Publish,
		})

	suite.Equal(suite.securityError, err)
	suite.Assert().Empty(publications)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_ExecuteScenarioPublicationAction_require_preparation() {
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId, suite.iterationId).Return(
		suite.existingIndexes, 0, nil)

	_, err := suite.makeUsecase().ExecuteScenarioPublicationAction(
		suite.ctx,
		suite.organizationId,
		models.PublishScenarioIterationInput{
			ScenarioIterationId: suite.iterationId,
			PublicationAction:   models.Publish,
		})

	suite.Assert().ErrorIs(err, models.BadParameterError)

	suite.AssertExpectations()
}

// GetPublicationPreparationStatus
func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetPublicationPreparationStatus_nominal_none_to_create() {
	// no preparation is currently running
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId,
		suite.iterationId).Return([]models.ConcreteIndex{}, 0, nil)
	status, err := suite.makeUsecase().GetPublicationPreparationStatus(suite.ctx,
		suite.organizationId, suite.iterationId)

	suite.NoError(err)
	suite.Assert().Equal(models.PublicationPreparationStatus{
		PreparationStatus:        models.PreparationStatusReadyToActivate,
		PreparationServiceStatus: models.PreparationServiceStatusAvailable,
	}, status)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetPublicationPreparationStatus_nominal_already_running() {
	// another prepration is running
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId,
		suite.iterationId).Return([]models.ConcreteIndex{}, 1, nil)

	// {Indexed: []string{"a", "b"}, Included: []string{"c", "d"}},
	status, err := suite.makeUsecase().GetPublicationPreparationStatus(
		suite.ctx,
		suite.organizationId,
		suite.iterationId)

	suite.NoError(err)
	suite.Assert().Equal(models.PublicationPreparationStatus{
		PreparationStatus:        models.PreparationStatusReadyToActivate,
		PreparationServiceStatus: models.PreparationServiceStatusOccupied,
	}, status)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetPublicationPreparationStatus_nominal_3() {
	// One index to create
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId,
		suite.iterationId).Return([]models.ConcreteIndex{
		{Indexed: []string{"a", "b"}, Included: []string{"c", "d"}},
	}, 0, nil)

	status, err := suite.makeUsecase().GetPublicationPreparationStatus(
		suite.ctx,
		suite.organizationId,
		suite.iterationId)

	suite.NoError(err)
	suite.Assert().Equal(models.PublicationPreparationStatus{
		PreparationStatus:        models.PreparationStatusRequired,
		PreparationServiceStatus: models.PreparationServiceStatusAvailable,
	}, status)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetPublicationPreparationStatus_get_error() {
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId, suite.iterationId).Return(
		[]models.ConcreteIndex{}, 0, suite.repositoryError)

	_, err := suite.makeUsecase().GetPublicationPreparationStatus(suite.ctx,
		suite.organizationId, suite.iterationId)

	suite.ErrorIs(err, suite.repositoryError)

	suite.AssertExpectations()
}

// StartPublicationPreparation
func (suite *ScenarioPublicationUsecaseTestSuite) Test_StartPublicationPreparation_none_to_create() {
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId,
		suite.iterationId).Return([]models.ConcreteIndex{}, 0, nil)

	err := suite.makeUsecase().StartPublicationPreparation(suite.ctx, suite.organizationId, suite.iterationId)

	suite.NoError(err)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_StartPublicationPreparation_preparation_already_in_progress() {
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId,
		suite.iterationId).Return([]models.ConcreteIndex{
		{Indexed: []string{"a", "b"}, Included: []string{"c", "d"}},
	}, 1, nil)

	err := suite.makeUsecase().StartPublicationPreparation(suite.ctx, suite.organizationId, suite.iterationId)

	suite.Assert().ErrorIs(err, models.ConflictError)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_StartPublicationPreparation_preparation_nominal() {
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId,
		suite.iterationId).Return([]models.ConcreteIndex{
		{Indexed: []string{"a", "b"}, Included: []string{"c", "d"}},
	}, 0, nil)
	suite.clientDbIndexEditor.On("CreateIndexesAsync",
		suite.ctx,
		suite.organizationId,
		[]models.ConcreteIndex{
			{Indexed: []string{"a", "b"}, Included: []string{"c", "d"}},
		}).Return(nil)

	err := suite.makeUsecase().StartPublicationPreparation(suite.ctx, suite.organizationId, suite.iterationId)

	suite.NoError(err)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_StartPublicationPreparation_get_error() {
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId, suite.iterationId).Return(
		[]models.ConcreteIndex{}, 0, suite.repositoryError)

	err := suite.makeUsecase().StartPublicationPreparation(suite.ctx, suite.organizationId, suite.iterationId)

	suite.ErrorIs(err, suite.repositoryError)

	suite.AssertExpectations()
}

func TestScenarioPublicationUsecase(t *testing.T) {
	suite.Run(t, new(ScenarioPublicationUsecaseTestSuite))
}
