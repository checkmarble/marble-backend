package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type TaskQueueRepository struct {
	mock.Mock
}

func (m *TaskQueueRepository) EnqueueDecisionTask(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId string,
	decision models.DecisionToCreate,
	scenarioIterationId string,
) error {
	args := m.Called(ctx, tx, organizationId, decision, scenarioIterationId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueDecisionTaskMany(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId string,
	decisions []models.DecisionToCreate,
	scenarioIterationId string,
) error {
	args := m.Called(ctx, tx, organizationId, decisions, scenarioIterationId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueScheduledExecStatusTask(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId string,
	scheduledExecutionId string,
) error {
	args := m.Called(ctx, tx, organizationId, scheduledExecutionId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueCreateIndexTask(
	ctx context.Context,
	organizationId string,
	indices []models.ConcreteIndex,
) error {
	args := m.Called(ctx, organizationId, indices)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueMatchEnrichmentTask(
	ctx context.Context,
	organizationId string,
	sanctionCheckId string,
) error {
	args := m.Called(ctx, organizationId, sanctionCheckId)
	return args.Error(0)
}
