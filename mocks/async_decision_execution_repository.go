package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type AsyncDecisionExecutionRepository struct {
	mock.Mock
}

func (m *AsyncDecisionExecutionRepository) GetAsyncDecisionExecution(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
) (models.AsyncDecisionExecution, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.AsyncDecisionExecution), args.Error(1)
}

func (m *AsyncDecisionExecutionRepository) UpdateAsyncDecisionExecution(
	ctx context.Context,
	exec repositories.Executor,
	input models.AsyncDecisionExecutionUpdate,
) error {
	args := m.Called(ctx, exec, input)
	return args.Error(0)
}
