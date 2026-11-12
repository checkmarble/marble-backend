package usecases

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

type ApiKeyUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity    *mocks.EnforceSecurity
	executorFactory    *mocks.ExecutorFactory
	apiKeyRepository   *mocks.ApiKeyRepository
	grantRepository    *mocks.GrantRepository
	transactionFactory *mocks.TransactionFactory
	tx                 *mocks.Transaction

	organizationId  uuid.UUID
	repositoryError error
	securityError   error
}

func (suite *ApiKeyUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.apiKeyRepository = new(mocks.ApiKeyRepository)
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.tx = new(mocks.Transaction)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.tx}
	suite.grantRepository = new(mocks.GrantRepository)

	suite.organizationId = uuid.MustParse("25ab6323-1657-4a52-923a-ef6983fe4532")

	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
}

func (suite *ApiKeyUsecaseTestSuite) makeUsecase() *ApiKeyUseCase {
	return &ApiKeyUseCase{
		apiKeyRepository:   suite.apiKeyRepository,
		enforceSecurity:    suite.enforceSecurity,
		executorFactory:    suite.executorFactory,
		transactionFactory: suite.transactionFactory,
		grantRepository:    suite.grantRepository,
	}
}

func (suite *ApiKeyUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.apiKeyRepository.AssertExpectations(t)
	suite.enforceSecurity.AssertExpectations(t)
	suite.executorFactory.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
	suite.grantRepository.AssertExpectations(t)
}

func (suite *ApiKeyUsecaseTestSuite) Test_CreateApiKey_nominal() {
	ctx := context.Background()
	input := models.CreateApiKeyInput{
		OrganizationId: suite.organizationId,
		Description:    "test key", Role: models.API_CLIENT,
	}
	suite.transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
	suite.grantRepository.On("CreateApiKeyGrant", mock.Anything, suite.tx, mock.Anything, models.Grant{Role: models.API_CLIENT, OrganizationId: suite.organizationId}).Return(nil)
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId, models.API_CLIENT).Return(nil)
	suite.apiKeyRepository.On(
		"CreateApiKey",
		suite.tx,
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
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId, models.ADMIN).Return(nil)

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
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId, models.API_CLIENT).Return(suite.securityError)

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
	suite.transactionFactory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateApiKey", suite.organizationId, models.API_CLIENT).Return(nil)
	suite.apiKeyRepository.On("CreateApiKey", suite.tx,
		mock.AnythingOfType("models.ApiKey")).Return(suite.repositoryError)

	_, err := suite.makeUsecase().CreateApiKey(ctx, input)

	suite.ErrorIs(err, suite.repositoryError)

	suite.AssertExpectations()
}

func TestApiKeyUsecase(t *testing.T) {
	suite.Run(t, new(ApiKeyUsecaseTestSuite))
}

func TestCreateOIDCRecoveryKeyUsesTransaction(t *testing.T) {
	org := uuid.New()
	tenant := uuid.New()
	guard := new(mocks.EnforceSecurity)
	guard.On("CreateApiKey", org, models.OIDC_CONFIG_EDITOR).Return(nil)
	guard.On("CheckApiKeyTenant", tenant).Return(nil)
	tx := &mocks.Transaction{}
	factory := &mocks.TransactionFactory{TxMock: tx}
	factory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
	repo := new(mocks.ApiKeyRepository)
	orgRepo := new(mocks.OrganizationRepository)
	orgRepo.On("LockOrganizationTenant", mock.Anything, tx, org).Return(tenant, nil)
	grants := new(mocks.GrantRepository)
	grants.On("CreateApiKeyGrant", mock.Anything, tx, mock.Anything, models.Grant{Role: models.OIDC_CONFIG_EDITOR, TenantId: tenant}).Return(nil)
	repo.On("CreateApiKey", tx, mock.MatchedBy(func(key models.ApiKey) bool {
		return key.OrganizationId == org && key.Role == models.OIDC_CONFIG_EDITOR
	})).Return(nil)
	usecase := ApiKeyUseCase{transactionFactory: factory, enforceSecurity: guard, apiKeyRepository: repo, organizationRepository: orgRepo, grantRepository: grants}
	created, err := usecase.CreateApiKey(t.Context(), models.CreateApiKeyInput{OrganizationId: org, Role: models.OIDC_CONFIG_EDITOR})
	assert := suite.Suite{}
	assert.SetT(t)
	assert.NoError(err)
	assert.NotEmpty(created.Key)
	repo.AssertExpectations(t)
	factory.AssertExpectations(t)
}

func TestCreateAPIKeyReturnsGrantError(t *testing.T) {
	org := uuid.New()
	tx := &mocks.Transaction{}
	factory := &mocks.TransactionFactory{TxMock: tx}
	factory.On("Transaction", mock.Anything, mock.Anything).Return(nil)
	guard := new(mocks.EnforceSecurity)
	guard.On("CreateApiKey", org, models.API_CLIENT).Return(nil)
	repo := new(mocks.ApiKeyRepository)
	repo.On("CreateApiKey", tx, mock.Anything).Return(nil)
	grantErr := errors.New("grant insert failed")
	grants := new(mocks.GrantRepository)
	grants.On("CreateApiKeyGrant", mock.Anything, tx, mock.Anything, models.Grant{Role: models.API_CLIENT, OrganizationId: org}).Return(grantErr)
	usecase := ApiKeyUseCase{transactionFactory: factory, enforceSecurity: guard, apiKeyRepository: repo, grantRepository: grants}
	created, err := usecase.CreateApiKey(t.Context(), models.CreateApiKeyInput{OrganizationId: org, Role: models.API_CLIENT})
	if !errors.Is(err, grantErr) || created.Key != "" {
		t.Fatalf("expected grant error and no key, got %v and %+v", err, created)
	}
}

func TestCannotCreateOIDCEditorUser(t *testing.T) {
	usecase := UserUseCase{}
	_, err := usecase.AddUser(t.Context(), models.CreateUser{Role: models.OIDC_CONFIG_EDITOR, OrganizationId: uuid.New()})
	if !errors.Is(err, models.BadParameterError) {
		t.Fatalf("expected bad parameter, got %v", err)
	}
}
