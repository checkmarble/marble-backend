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
	organizationId uuid.UUID,
	decision models.DecisionToCreate,
	scenarioIterationId string,
) error {
	args := m.Called(ctx, tx, organizationId, decision, scenarioIterationId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueDecisionTaskMany(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	decisions []models.DecisionToCreate,
	scenarioIterationId string,
) error {
	args := m.Called(ctx, tx, organizationId, decisions, scenarioIterationId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueScheduledExecStatusTask(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	scheduledExecutionId string,
) error {
	args := m.Called(ctx, tx, organizationId, scheduledExecutionId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueCreateIndexTask(
	ctx context.Context,
	organizationId uuid.UUID,
	indices []models.ConcreteIndex,
) error {
	args := m.Called(ctx, organizationId, indices)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueMatchEnrichmentTask(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	screeningId string,
) error {
	args := m.Called(ctx, tx, organizationId, screeningId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueCaseReviewTask(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	caseId uuid.UUID,
	aiCaseReviewId uuid.UUID,
) error {
	args := m.Called(ctx, tx, organizationId, caseId, aiCaseReviewId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueAutoAssignmentTask(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	inboxId uuid.UUID,
) error {
	return m.Called(ctx, tx, orgId, inboxId).Error(0)
}

func (m *TaskQueueRepository) EnqueueDecisionWorkflowTask(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	decisionId string,
) error {
	return m.Called(ctx, tx, orgId, decisionId).Error(0)
}

func (m *TaskQueueRepository) EnqueueSendBillingEventTask(
	ctx context.Context,
	event models.BillingEvent,
) error {
	return m.Called(ctx, event).Error(0)
}

func (m *TaskQueueRepository) EnqueueContinuousScreeningDoScreeningTaskMany(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	objectType string,
	enqueueObjectUpdateTasks []models.ContinuousScreeningEnqueueObjectUpdateTask,
	triggerType models.ContinuousScreeningTriggerType,
) error {
	args := m.Called(ctx, tx, orgId, objectType, enqueueObjectUpdateTasks, triggerType)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueContinuousScreeningApplyDeltaFileTask(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	updateId uuid.UUID,
) error {
	args := m.Called(ctx, tx, orgId, updateId)
	return args.Error(0)
}
