package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ScenarioTestrunTestSuite struct {
	suite.Suite
	transactionFactory *mocks.TransactionFactory
	transaction        *mocks.Transaction
	// exec                           *mocks.Executor
	executorFactory                *mocks.ExecutorFactory
	scenarioPublicationsRepository *mocks.ScenarioPublicationRepository
	enforceSecurity                *mocks.EnforceSecurity
	repository                     *mocks.ScenatioTestrunRepository
	organizationId                 string
	scenarioId                     string
	scenarioPublicationId          string
	scenarioPublication            models.ScenarioPublication
	ctx                            context.Context
}

func (suite *ScenarioTestrunTestSuite) SetupTest() {
	// suite.exec = new(mocks.Executor)
	suite.transaction = new(mocks.Transaction)
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.scenarioPublicationsRepository = new(mocks.ScenarioPublicationRepository)
	suite.repository = new(mocks.ScenatioTestrunRepository)
	suite.organizationId = "25ab6323-1657-4a52-923a-ef6983fe4532"
	suite.scenarioId = "c5968ff7-6142-4623-a6b3-1539f345e5fa"
	suite.scenarioPublicationId = "c1c005f5-a920-4f92-aee1-f5007f2ad8c1"
	suite.ctx = context.Background()
	suite.scenarioPublication = models.ScenarioPublication{
		OrganizationId:      suite.organizationId,
		ScenarioId:          suite.scenarioId,
		ScenarioIterationId: suite.scenarioPublicationId,
	}
}

func (suite *ScenarioTestrunTestSuite) makeUsecase() *ScenarioTestRunUsecase {
	return &ScenarioTestRunUsecase{
		transactionFactory:             suite.transactionFactory,
		executorFactory:                suite.executorFactory,
		enforceSecurity:                suite.enforceSecurity,
		repository:                     suite.repository,
		scenarioPublicationsRepository: suite.scenarioPublicationsRepository,
	}
}

func (suite *ScenarioTestrunTestSuite) TestActivateScenarioTestRun() {
	input := models.ScenarioTestRunInput{
		ScenarioIterationId: "b53fcdd9-4909-4167-9b22-7e36a065ffbd",
		ScenarioId:          "b6f0c253-ca06-4a5c-a208-9d5a537ca827",
		Period:              time.Duration(10),
	}
	output := models.ScenarioTestRun{
		ScenarioIterationId: "b53fcdd9-4909-4167-9b22-7e36a065ffbd",
		ScenarioId:          "b6f0c253-ca06-4a5c-a208-9d5a537ca827",
		Status:              models.Up,
		Period:              time.Duration(1000),
	}
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.scenarioPublicationsRepository.On("ListScenarioPublicationsOfOrganization", suite.ctx,
		suite.transaction, suite.organizationId, models.ListScenarioPublicationsFilters{
			ScenarioId: &input.ScenarioId,
		}).Return([]models.ScenarioPublication{
		suite.scenarioPublication,
	}, nil)
	suite.repository.On("GetByID", suite.ctx, mock.Anything).Return(&models.ScenarioTestRun{
		ScenarioIterationId: output.ScenarioIterationId,
	}, nil)
	suite.repository.On("CreateTestRun", suite.ctx, suite.transaction, mock.Anything, input).Return(nil)
	suite.repository.On("GetByScenarioIterationID", suite.ctx, suite.transaction,
		input.ScenarioIterationId).Return(models.ScenarioTestRun{}, nil)
	suite.repository.On("GetByID", suite.ctx, suite.transaction,
		mock.Anything).Return(output, nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	result, err := suite.makeUsecase().ActivateScenarioTestRun(suite.ctx, suite.organizationId, input)
	t := suite.T()
	assert.Equal(t, nil, err)
	assert.Equal(t, result.ScenarioIterationId, output.ScenarioIterationId)
}

func TestScenationTestrun(t *testing.T) {
	suite.Run(t, new(ScenarioTestrunTestSuite))
}
