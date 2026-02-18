package webhooks

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

// Mock for enforceSecurityWebhook interface
type mockEnforceSecurityWebhook struct {
	mock.Mock
}

func (m *mockEnforceSecurityWebhook) CanCreateWebhook(ctx context.Context, organizationId uuid.UUID, partnerId null.String) error {
	args := m.Called(ctx, organizationId, partnerId)
	return args.Error(0)
}

func (m *mockEnforceSecurityWebhook) CanReadWebhook(ctx context.Context, webhook models.Webhook) error {
	args := m.Called(ctx, webhook)
	return args.Error(0)
}

func (m *mockEnforceSecurityWebhook) CanModifyWebhook(ctx context.Context, webhook models.Webhook) error {
	args := m.Called(ctx, webhook)
	return args.Error(0)
}

type WebhooksUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity    *mockEnforceSecurityWebhook
	exec               *mocks.Executor
	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
	executorFactory    *mocks.ExecutorFactory
	webhookRepository  *mocks.WebhooksRepository

	organizationId  uuid.UUID
	webhookId       uuid.UUID
	secretId        uuid.UUID
	repositoryError error
	securityError   error
	ctx             context.Context
}

func (suite *WebhooksUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mockEnforceSecurityWebhook)
	suite.webhookRepository = new(mocks.WebhooksRepository)
	suite.exec = new(mocks.Executor)
	suite.transaction = new(mocks.Transaction)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.executorFactory = new(mocks.ExecutorFactory)

	suite.organizationId = uuid.MustParse("25ab6323-1657-4a52-923a-ef6983fe4532")
	suite.webhookId = uuid.Must(uuid.NewV7())
	suite.secretId = uuid.Must(uuid.NewV7())

	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
	suite.ctx = context.Background()
}

func (suite *WebhooksUsecaseTestSuite) makeUsecase() WebhooksUsecase {
	return WebhooksUsecase{
		enforceSecurity:       suite.enforceSecurity,
		executorFactory:       suite.executorFactory,
		transactionFactory:    suite.transactionFactory,
		webhookRepository:     suite.webhookRepository,
		webhookSystemMigrated: true,
	}
}

func (suite *WebhooksUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.webhookRepository.AssertExpectations(t)
	suite.exec.AssertExpectations(t)
	suite.transaction.AssertExpectations(t)
}

// CreateWebhookSecret tests

func (suite *WebhooksUsecaseTestSuite) Test_CreateWebhookSecret_nominal() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.webhookRepository.On("AddWebhookSecret", suite.ctx, suite.transaction,
		mock.AnythingOfType("models.NewWebhookSecret")).Return(nil)

	secret, err := suite.makeUsecase().CreateWebhookSecret(suite.ctx, suite.webhookId, nil)

	t := suite.T()
	assert.NoError(t, err)
	assert.NotEmpty(t, secret.Value)
	assert.NotEmpty(t, secret.Uid)

	suite.AssertExpectations()
}

func (suite *WebhooksUsecaseTestSuite) Test_CreateWebhookSecret_with_expiration() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.webhookRepository.On("ExpireWebhookSecrets", suite.ctx, suite.transaction,
		suite.webhookId, mock.AnythingOfType("time.Time")).Return(nil)
	suite.webhookRepository.On("AddWebhookSecret", suite.ctx, suite.transaction,
		mock.AnythingOfType("models.NewWebhookSecret")).Return(nil)

	expireDays := 7
	secret, err := suite.makeUsecase().CreateWebhookSecret(suite.ctx, suite.webhookId, &expireDays)

	t := suite.T()
	assert.NoError(t, err)
	assert.NotEmpty(t, secret.Value)

	suite.AssertExpectations()
}

func (suite *WebhooksUsecaseTestSuite) Test_CreateWebhookSecret_not_migrated() {
	usecase := WebhooksUsecase{webhookSystemMigrated: false}

	_, err := usecase.CreateWebhookSecret(suite.ctx, suite.webhookId, nil)

	t := suite.T()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only available for migrated webhooks")
}

func (suite *WebhooksUsecaseTestSuite) Test_CreateWebhookSecret_webhook_not_found() {
	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).
		Return(models.NewWebhook{}, models.NotFoundError)

	_, err := suite.makeUsecase().CreateWebhookSecret(suite.ctx, suite.webhookId, nil)

	t := suite.T()
	assert.ErrorIs(t, err, models.NotFoundError)

	suite.AssertExpectations()
}

func (suite *WebhooksUsecaseTestSuite) Test_CreateWebhookSecret_security_error() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).
		Return(models.ForbiddenError)

	_, err := suite.makeUsecase().CreateWebhookSecret(suite.ctx, suite.webhookId, nil)

	t := suite.T()
	assert.ErrorIs(t, err, models.ForbiddenError)

	suite.AssertExpectations()
}

// RevokeWebhookSecret tests

func (suite *WebhooksUsecaseTestSuite) Test_RevokeWebhookSecret_nominal() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}
	secret := models.NewWebhookSecret{
		Id:        suite.secretId,
		WebhookId: suite.webhookId,
		Value:     "test-secret",
		ExpiresAt: nil, // permanent secret
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).Return(nil)
	suite.webhookRepository.On("GetWebhookSecret", suite.ctx, suite.exec, suite.secretId).Return(secret, nil)
	suite.webhookRepository.On("CountPermanentWebhookSecrets", suite.ctx, suite.exec, suite.webhookId).Return(2, nil)
	suite.webhookRepository.On("RevokeWebhookSecret", suite.ctx, suite.exec, suite.secretId).Return(nil)

	err := suite.makeUsecase().RevokeWebhookSecret(suite.ctx, suite.webhookId, suite.secretId)

	t := suite.T()
	assert.NoError(t, err)

	suite.AssertExpectations()
}

func (suite *WebhooksUsecaseTestSuite) Test_RevokeWebhookSecret_expiring_with_one_permanent() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	secret := models.NewWebhookSecret{
		Id:        suite.secretId,
		WebhookId: suite.webhookId,
		Value:     "test-secret",
		ExpiresAt: &expiresAt, // expiring secret
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).Return(nil)
	suite.webhookRepository.On("GetWebhookSecret", suite.ctx, suite.exec, suite.secretId).Return(secret, nil)
	suite.webhookRepository.On("CountPermanentWebhookSecrets", suite.ctx, suite.exec, suite.webhookId).Return(1, nil)
	suite.webhookRepository.On("RevokeWebhookSecret", suite.ctx, suite.exec, suite.secretId).Return(nil)

	err := suite.makeUsecase().RevokeWebhookSecret(suite.ctx, suite.webhookId, suite.secretId)

	t := suite.T()
	assert.NoError(t, err)

	suite.AssertExpectations()
}

func (suite *WebhooksUsecaseTestSuite) Test_RevokeWebhookSecret_fails_last_permanent() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}
	secret := models.NewWebhookSecret{
		Id:        suite.secretId,
		WebhookId: suite.webhookId,
		Value:     "test-secret",
		ExpiresAt: nil, // permanent secret
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).Return(nil)
	suite.webhookRepository.On("GetWebhookSecret", suite.ctx, suite.exec, suite.secretId).Return(secret, nil)
	suite.webhookRepository.On("CountPermanentWebhookSecrets", suite.ctx, suite.exec, suite.webhookId).Return(1, nil)

	err := suite.makeUsecase().RevokeWebhookSecret(suite.ctx, suite.webhookId, suite.secretId)

	t := suite.T()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must retain at least one permanent")

	suite.AssertExpectations()
}

func (suite *WebhooksUsecaseTestSuite) Test_RevokeWebhookSecret_fails_expiring_with_no_permanent() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	secret := models.NewWebhookSecret{
		Id:        suite.secretId,
		WebhookId: suite.webhookId,
		Value:     "test-secret",
		ExpiresAt: &expiresAt, // expiring secret
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).Return(nil)
	suite.webhookRepository.On("GetWebhookSecret", suite.ctx, suite.exec, suite.secretId).Return(secret, nil)
	suite.webhookRepository.On("CountPermanentWebhookSecrets", suite.ctx, suite.exec, suite.webhookId).Return(0, nil)

	err := suite.makeUsecase().RevokeWebhookSecret(suite.ctx, suite.webhookId, suite.secretId)

	t := suite.T()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must retain at least one permanent")

	suite.AssertExpectations()
}

func (suite *WebhooksUsecaseTestSuite) Test_RevokeWebhookSecret_not_migrated() {
	usecase := WebhooksUsecase{webhookSystemMigrated: false}

	err := usecase.RevokeWebhookSecret(suite.ctx, suite.webhookId, suite.secretId)

	t := suite.T()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only available for migrated webhooks")
}

func (suite *WebhooksUsecaseTestSuite) Test_RevokeWebhookSecret_secret_wrong_webhook() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}
	otherWebhookId := uuid.Must(uuid.NewV7())
	secret := models.NewWebhookSecret{
		Id:        suite.secretId,
		WebhookId: otherWebhookId, // belongs to different webhook
		Value:     "test-secret",
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).Return(nil)
	suite.webhookRepository.On("GetWebhookSecret", suite.ctx, suite.exec, suite.secretId).Return(secret, nil)

	err := suite.makeUsecase().RevokeWebhookSecret(suite.ctx, suite.webhookId, suite.secretId)

	t := suite.T()
	assert.ErrorIs(t, err, models.NotFoundError)

	suite.AssertExpectations()
}

func (suite *WebhooksUsecaseTestSuite) Test_RevokeWebhookSecret_already_revoked() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}
	revokedAt := time.Now().Add(-time.Hour)
	secret := models.NewWebhookSecret{
		Id:        suite.secretId,
		WebhookId: suite.webhookId,
		Value:     "test-secret",
		RevokedAt: &revokedAt, // already revoked
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).Return(nil)
	suite.webhookRepository.On("GetWebhookSecret", suite.ctx, suite.exec, suite.secretId).Return(secret, nil)

	err := suite.makeUsecase().RevokeWebhookSecret(suite.ctx, suite.webhookId, suite.secretId)

	t := suite.T()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already revoked")

	suite.AssertExpectations()
}

func (suite *WebhooksUsecaseTestSuite) Test_RevokeWebhookSecret_security_error() {
	webhook := models.NewWebhook{
		Id:             suite.webhookId,
		OrganizationId: suite.organizationId,
		Url:            "https://example.com/webhook",
	}

	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.webhookRepository.On("GetWebhook", suite.ctx, suite.exec, suite.webhookId).Return(webhook, nil)
	suite.enforceSecurity.On("CanModifyWebhook", suite.ctx, mock.AnythingOfType("models.Webhook")).
		Return(models.ForbiddenError)

	err := suite.makeUsecase().RevokeWebhookSecret(suite.ctx, suite.webhookId, suite.secretId)

	t := suite.T()
	assert.ErrorIs(t, err, models.ForbiddenError)

	suite.AssertExpectations()
}

func TestWebhooksUsecase(t *testing.T) {
	suite.Run(t, new(WebhooksUsecaseTestSuite))
}
