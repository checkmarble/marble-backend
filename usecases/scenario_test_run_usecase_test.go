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

	executorFactory       *mocks.ExecutorFactory
	scenarioRepository    *mocks.ScenarioRepository
	enforceSecurity       *mocks.EnforceSecurity
	repository            *mocks.ScenarioTestrunRepository
	clientDbIndexEditor   *mocks.ClientDbIndexEditor
	featureAccessReader   *mocks.FeatureAccessReader
	sanctionCheckConfig   *mocks.SanctionCheckConfigRepository
	organizationId        string
	scenarioId            string
	scenarioPublicationId string
	ctx                   context.Context
}

func (suite *ScenarioTestrunTestSuite) SetupTest() {
	suite.transaction = new(mocks.Transaction)
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.scenarioRepository = new(mocks.ScenarioRepository)
	suite.repository = new(mocks.ScenarioTestrunRepository)
	suite.featureAccessReader = new(mocks.FeatureAccessReader)
	suite.sanctionCheckConfig = new(mocks.SanctionCheckConfigRepository)
	suite.organizationId = "25ab6323-1657-4a52-923a-ef6983fe4532"
	suite.scenarioId = "c5968ff7-6142-4623-a6b3-1539f345e5fa"
	suite.scenarioPublicationId = "c1c005f5-a920-4f92-aee1-f5007f2ad8c1"
	suite.clientDbIndexEditor = new(mocks.ClientDbIndexEditor)
	suite.ctx = context.Background()
}

func (suite *ScenarioTestrunTestSuite) makeUsecase() *ScenarioTestRunUsecase {
	return &ScenarioTestRunUsecase{
		transactionFactory:            suite.transactionFactory,
		executorFactory:               suite.executorFactory,
		enforceSecurity:               suite.enforceSecurity,
		repository:                    suite.repository,
		scenarioRepository:            suite.scenarioRepository,
		clientDbIndexEditor:           suite.clientDbIndexEditor,
		featureAccessReader:           suite.featureAccessReader,
		sanctionCheckConfigRepository: suite.sanctionCheckConfig,
	}
}

func (suite *ScenarioTestrunTestSuite) TestActivateScenarioTestRun() {
	input := models.ScenarioTestRunInput{
		PhantomIterationId: "b53fcdd9-4909-4167-9b22-7e36a065ffbd",
		ScenarioId:         "b6f0c253-ca06-4a5c-a208-9d5a537ca827",
		EndDate:            time.Now(),
	}
	output := models.ScenarioTestRun{
		ScenarioIterationId: "b53fcdd9-4909-4167-9b22-7e36a065ffbd",
		ScenarioId:          "b6f0c253-ca06-4a5c-a208-9d5a537ca827",
		Status:              models.Up,
	}
	liveVersionID := "b76359b2-9806-40f1-9fee-7ea18c797b2e"
	suite.clientDbIndexEditor.On("GetIndexesToCreate", suite.ctx, suite.organizationId, mock.Anything).Return(
		[]models.ConcreteIndex{
			{
				TableName: "sample_table",
			},
		}, 0, nil)
	suite.clientDbIndexEditor.On("CreateIndexes",
		suite.ctx,
		suite.organizationId,
		mock.Anything).Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.scenarioRepository.On("GetScenarioById",
		suite.transaction, input.ScenarioId).Return(models.Scenario{
		LiveVersionID: &liveVersionID,
	}, nil)
	suite.repository.On("GetTestRunByID", suite.ctx, suite.transaction,
		mock.Anything).Return(output, nil)
	suite.enforceSecurity.On("CreateTestRun", suite.organizationId).Return(nil)
	suite.repository.On("CreateTestRun", suite.ctx, suite.transaction, mock.Anything,
		input.CreateDbInput(liveVersionID)).Return(nil)
	suite.repository.On("ListRunningTestRun", suite.ctx, suite.transaction,
		suite.organizationId).Return(nil, nil)
	suite.sanctionCheckConfig.On("ListSanctionCheckConfigs", suite.ctx, suite.transaction,
		input.PhantomIterationId).Return(nil, nil)

	suite.clientDbIndexEditor.On("CreateIndexesAsyncForScenarioWithCallback", suite.ctx,
		suite.organizationId, []models.ConcreteIndex{
			{
				TableName: "sample_table",
			},
		}, mock.Anything,
	).Return(nil)

	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	result, err := suite.makeUsecase().CreateScenarioTestRun(suite.ctx, suite.organizationId, input)
	t := suite.T()
	assert.Equal(t, nil, err)
	assert.Equal(t, result.ScenarioIterationId, output.ScenarioIterationId)
}

func TestScenationTestrun(t *testing.T) {
	suite.Run(t, new(ScenarioTestrunTestSuite))
}
