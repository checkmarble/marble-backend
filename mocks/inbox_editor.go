package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type InboxEditor struct {
	mock.Mock
}

func (m *InboxEditor) CreateInbox(ctx context.Context, input models.CreateInboxInput) (models.Inbox, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(models.Inbox), args.Error(1)
}
