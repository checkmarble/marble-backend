package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type WebhookEventsUsecase struct {
	mock.Mock
}

func (m *WebhookEventsUsecase) CreateWebhookEvent(
	ctx context.Context,
	tx repositories.Transaction,
	input models.WebhookEventCreate,
) error {
	args := m.Called(ctx, tx, input)
	return args.Error(0)
}

func (m *WebhookEventsUsecase) SendWebhookEventAsync(ctx context.Context, webhookEventId string) {
	m.Called(ctx, webhookEventId)
}
