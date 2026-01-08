package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type InboxReader struct {
	mock.Mock
}

func (m *InboxReader) GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error) {
	args := m.Called(ctx, exec, inboxId)
	return args.Get(0).(models.Inbox), args.Error(1)
}

func (m *InboxReader) ListInboxes(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, withCaseCount bool) ([]models.Inbox, error) {
	args := m.Called(ctx, exec, orgId, withCaseCount)
	return args.Get(0).([]models.Inbox), args.Error(1)
}
