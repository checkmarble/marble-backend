package mocks

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type WebhooksRepository struct {
	mock.Mock
}

func (m *WebhooksRepository) CreateWebhook(ctx context.Context, exec repositories.Executor, webhook models.NewWebhook) error {
	args := m.Called(ctx, exec, webhook)
	return args.Error(0)
}

func (m *WebhooksRepository) GetWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.NewWebhook, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.NewWebhook), args.Error(1)
}

func (m *WebhooksRepository) ListWebhooks(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) ([]models.NewWebhook, error) {
	args := m.Called(ctx, exec, orgId)
	return args.Get(0).([]models.NewWebhook), args.Error(1)
}

func (m *WebhooksRepository) UpdateWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID, update models.NewWebhookUpdate) error {
	args := m.Called(ctx, exec, id, update)
	return args.Error(0)
}

func (m *WebhooksRepository) DeleteWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) error {
	args := m.Called(ctx, exec, id)
	return args.Error(0)
}

func (m *WebhooksRepository) AddWebhookSecret(ctx context.Context, exec repositories.Executor, secret models.NewWebhookSecret) error {
	args := m.Called(ctx, exec, secret)
	return args.Error(0)
}

func (m *WebhooksRepository) ListActiveWebhookSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID) ([]models.NewWebhookSecret, error) {
	args := m.Called(ctx, exec, webhookId)
	return args.Get(0).([]models.NewWebhookSecret), args.Error(1)
}

func (m *WebhooksRepository) GetWebhookSecret(ctx context.Context, exec repositories.Executor, secretId uuid.UUID) (models.NewWebhookSecret, error) {
	args := m.Called(ctx, exec, secretId)
	return args.Get(0).(models.NewWebhookSecret), args.Error(1)
}

func (m *WebhooksRepository) RevokeWebhookSecret(ctx context.Context, exec repositories.Executor, secretId uuid.UUID) error {
	args := m.Called(ctx, exec, secretId)
	return args.Error(0)
}

func (m *WebhooksRepository) ExpireWebhookSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID, expiresAt time.Time) error {
	args := m.Called(ctx, exec, webhookId, expiresAt)
	return args.Error(0)
}

func (m *WebhooksRepository) CountPermanentWebhookSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID) (int, error) {
	args := m.Called(ctx, exec, webhookId)
	return args.Int(0), args.Error(1)
}
