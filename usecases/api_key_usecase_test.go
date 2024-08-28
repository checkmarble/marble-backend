package usecases

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

type ApiKeyUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity  *mocks.EnforceSecurity
	transaction      *mocks.Executor
	executorFactory  *mocks.ExecutorFactory
	apiKeyRepository *mocks.ApiKeyRepository

	organizationId  string
	repositoryError error
	securityError   error
}

func (suite *ApiKeyUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.apiKeyRepository = new(mocks.ApiKeyRepository)
	suite.transaction = new(mocks.Executor)
	suite.executorFactory = new(mocks.ExecutorFactory)

	suite.organizationId = "25ab6323-1657-4a52-923a-ef6983fe4532"

	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
}

func (suite *ApiKeyUsecaseTestSuite) makeUsecase() *ApiKeyUseCase {
	return &ApiKeyUseCase{
		apiKeyRepository: suite.apiKeyRepository,
		enforceSecurity:  suite.enforceSecurity,
		executorFactory:  suite.executorFactory,
	}
}

func (suite *ApiKeyUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.apiKeyRepository.AssertExpectations(t)
	suite.enforceSecurity.AssertExpectations(t)
	suite.executorFactory.AssertExpectations(t)
}

func (suite *ApiKeyUsecaseTestSuite) Test_CreateApiKey_nominal() {
	ctx := context.Background()
	input := models.CreateApiKeyInput{
		OrganizationId: suite.organizationId,
		Description:    "test key", Role: models.API_CLIENT,
	}
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId).Return(nil)
	suite.apiKeyRepository.On(
		"CreateApiKey",
		suite.transaction,
		mock.AnythingOfType("models.ApiKey"),
	).
		Return(nil)

	createdApiKey, err := suite.makeUsecase().CreateApiKey(ctx, input)

	suite.NoError(err)
	suite.Equal(64, len(createdApiKey.Key))
	hash := sha256.Sum256([]byte(createdApiKey.Key))
	suite.Equal(createdApiKey.Hash, hash[:])
	suite.AssertExpectations()
}

func (suite *ApiKeyUsecaseTestSuite) Test_CreateApiKey_bad_parameter() {
	ctx := context.Background()
	input := models.CreateApiKeyInput{
		OrganizationId: suite.organizationId,
		Description:    "test key", Role: models.ADMIN,
	}
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId).Return(nil)

	_, err := suite.makeUsecase().CreateApiKey(ctx, input)

	suite.ErrorIs(err, models.BadParameterError)

	suite.AssertExpectations()
}

func (suite *ApiKeyUsecaseTestSuite) Test_CreateApiKey_security_error() {
	ctx := context.Background()
	input := models.CreateApiKeyInput{
		OrganizationId: suite.organizationId,
		Description:    "test key", Role: models.API_CLIENT,
	}
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId).Return(suite.securityError)

	_, err := suite.makeUsecase().CreateApiKey(ctx, input)

	suite.ErrorIs(err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *ApiKeyUsecaseTestSuite) Test_CreateApiKey_repository_error() {
	ctx := context.Background()
	input := models.CreateApiKeyInput{
		OrganizationId: suite.organizationId,
		Description:    "test key", Role: models.API_CLIENT,
	}
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId).Return(nil)
	suite.apiKeyRepository.On("CreateApiKey", suite.transaction,
		mock.AnythingOfType("models.ApiKey")).Return(suite.repositoryError)

	_, err := suite.makeUsecase().CreateApiKey(ctx, input)

	suite.ErrorIs(err, suite.repositoryError)

	suite.AssertExpectations()
}

func TestApiKeyUsecase(t *testing.T) {
	suite.Run(t, new(ApiKeyUsecaseTestSuite))
}
