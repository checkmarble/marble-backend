package usecases

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

type ApiKeyUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity    *mocks.EnforceSecurity
	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
	apiKeyRepository   *mocks.ApiKeyRepository

	organizationId  string
	apiKey          models.ApiKey
	createdApiKey   models.CreatedApiKey
	repositoryError error
	securityError   error
}

func (suite *ApiKeyUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.apiKeyRepository = new(mocks.ApiKeyRepository)
	suite.transaction = new(mocks.Transaction)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}

	suite.organizationId = "25ab6323-1657-4a52-923a-ef6983fe4532"
	suite.apiKey = models.ApiKey{
		Id:             "0ae6fda7-f7b3-4218-9fc3-4efa329432a7",
		OrganizationId: suite.organizationId,
		Description:    "test key",
		Role:           models.API_CLIENT,
	}
	suite.createdApiKey = models.CreatedApiKey{
		ApiKey: suite.apiKey,
		Value:  mock.Anything,
	}
	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
}

func (suite *ApiKeyUsecaseTestSuite) makeUsecase() *ApiKeyUseCase {
	return &ApiKeyUseCase{
		transactionFactory: suite.transactionFactory,
		organizationIdOfContext: func() (string, error) {
			return suite.organizationId, nil
		},
		enforceSecurity:  suite.enforceSecurity,
		apiKeyRepository: suite.apiKeyRepository,
	}
}

func (suite *ApiKeyUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.apiKeyRepository.AssertExpectations(t)
}

func (suite *ApiKeyUsecaseTestSuite) Test_CreateApiKey_nominal() {
	input := models.CreateApiKeyInput{OrganizationId: suite.organizationId, Description: "test key", Role: models.API_CLIENT}
	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId).Return(nil)
	suite.apiKeyRepository.On("CreateApiKey", suite.transaction, mock.AnythingOfType("models.CreateApiKey")).Return(nil)
	suite.apiKeyRepository.On("GetApiKeyById", suite.transaction, mock.AnythingOfType("string")).Return(suite.apiKey, nil)

	createdApiKey, err := suite.makeUsecase().CreateApiKey(context.Background(), input)

	suite.NoError(err)
	suite.Assert().NotEmpty(createdApiKey.Value)
	createdApiKey.Value = mock.Anything
	suite.Equal(suite.createdApiKey, createdApiKey)

	suite.AssertExpectations()
}

func (suite *ApiKeyUsecaseTestSuite) Test_CreateApiKey_security_error() {
	input := models.CreateApiKeyInput{OrganizationId: suite.organizationId, Description: "test key", Role: models.API_CLIENT}
	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId).Return(suite.securityError)

	_, err := suite.makeUsecase().CreateApiKey(context.Background(), input)

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *ApiKeyUsecaseTestSuite) Test_CreateApiKey_repository_error() {
	input := models.CreateApiKeyInput{OrganizationId: suite.organizationId, Description: "test key", Role: models.API_CLIENT}
	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId).Return(nil)
	suite.apiKeyRepository.On("CreateApiKey", suite.transaction, mock.AnythingOfType("models.CreateApiKey")).Return(suite.repositoryError)

	_, err := suite.makeUsecase().CreateApiKey(context.Background(), input)

	t := suite.T()
	assert.ErrorIs(t, err, suite.repositoryError)

	suite.AssertExpectations()
}

func TestApiKeyUsecase(t *testing.T) {
	suite.Run(t, new(ApiKeyUsecaseTestSuite))
}
