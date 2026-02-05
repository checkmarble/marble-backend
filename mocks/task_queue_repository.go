package mocks

import (
	"context"
	"time"

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
	orgId uuid.UUID,
	decision models.DecisionToCreate,
	scenarioIterationId string,
) error {
	return m.Called(ctx, tx, orgId, decision, scenarioIterationId).Error(0)
}

func (m *TaskQueueRepository) EnqueueDecisionTaskMany(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	decisions []models.DecisionToCreate,
	scenarioIterationId string,
) error {
	return m.Called(ctx, tx, orgId, decisions, scenarioIterationId).Error(0)
}

func (m *TaskQueueRepository) EnqueueScheduledExecStatusTask(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	scheduledExecutionId string,
) error {
	return m.Called(ctx, tx, orgId, scheduledExecutionId).Error(0)
}

func (m *TaskQueueRepository) EnqueueCreateIndexTask(
	ctx context.Context,
	orgId uuid.UUID,
	indices []models.ConcreteIndex,
) error {
	return m.Called(ctx, orgId, indices).Error(0)
}

func (m *TaskQueueRepository) EnqueueMatchEnrichmentTask(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	screeningId string,
) error {
	return m.Called(ctx, tx, orgId, screeningId).Error(0)
}

func (m *TaskQueueRepository) EnqueueCaseReviewTask(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	caseId uuid.UUID,
	aiCaseReviewId uuid.UUID,
) error {
	return m.Called(ctx, tx, orgId, caseId, aiCaseReviewId).Error(0)
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

func (m *TaskQueueRepository) EnqueueContinuousScreeningMatchEnrichmentTask(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	continuousScreeningId uuid.UUID,
) error {
	return m.Called(ctx, tx, orgId, continuousScreeningId).Error(0)
}

func (m *TaskQueueRepository) EnqueueContinuousScreeningApplyDeltaFileTask(
	ctx context.Context,
	tx repositories.Transaction,
	orgId uuid.UUID,
	updateJobId uuid.UUID,
) error {
	return m.Called(ctx, tx, orgId, updateJobId).Error(0)
}

func (m *TaskQueueRepository) EnqueueCsvIngestionTask(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	uploadLogId string,
	ingestionOptions models.IngestionOptions,
) error {
	args := m.Called(ctx, tx, organizationId, uploadLogId, ingestionOptions)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueScheduledExecutionTask(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	scheduledExecutionId string,
) error {
	args := m.Called(ctx, tx, organizationId, scheduledExecutionId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueGenerateThumbnailTask(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	bucket, key string,
) error {
	args := m.Called(ctx, tx, organizationId, bucket, key)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueWebhookDispatch(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	webhookEventId uuid.UUID,
) error {
	args := m.Called(ctx, tx, organizationId, webhookEventId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueWebhookDelivery(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	deliveryId uuid.UUID,
) error {
	args := m.Called(ctx, tx, organizationId, deliveryId)
	return args.Error(0)
}

func (m *TaskQueueRepository) EnqueueWebhookDeliveryAt(
	ctx context.Context,
	tx repositories.Transaction,
	organizationId uuid.UUID,
	deliveryId uuid.UUID,
	scheduledAt time.Time,
) error {
	args := m.Called(ctx, tx, organizationId, deliveryId, scheduledAt)
	return args.Error(0)
}
