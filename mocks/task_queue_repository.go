package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
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
	tx repositories.Transaction,
	organizationId string,
	screeningId string,
) error {
	args := m.Called(ctx, tx, organizationId, screeningId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueCaseReviewTask(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId string,
	caseId uuid.UUID,
	aiCaseReviewId uuid.UUID,
) error {
	args := m.Called(ctx, tx, organizationId, caseId, aiCaseReviewId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueAutoAssignmentTask(
	ctx context.Context,
	tx repositories.Transaction,
	orgId string, inboxId uuid.UUID,
) error {
	return m.Called(ctx, tx, orgId, inboxId).Error(0)
}
