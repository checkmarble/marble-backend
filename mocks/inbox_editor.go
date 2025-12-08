package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type InboxEditor struct {
	mock.Mock
}

func (m *InboxEditor) CreateInboxWithExecutor(
	ctx context.Context,
	exec repositories.Executor,
	input models.CreateInboxInput,
) (models.Inbox, error) {
	args := m.Called(ctx, exec, input)
	return args.Get(0).(models.Inbox), args.Error(1)
}
