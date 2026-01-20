package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

const (
	nbRetriesAsyncDecision       = 6 // at 1sec*attempt^4, that's 90min for the 6th attempt
	priorityAsyncDecision        = 3 // nb: higher number is lower priority (between 1 and 4)
	nbRetriesScheduledExecStatus = 7 // at 1sec*attempt^4, that's 6h for the 7th attempt
	priorityScheduledExecStatus  = 2
)

type TaskQueueRepository interface {
	EnqueueDecisionTask(
		ctx context.Context,
		tx Transaction,
		organizationId uuid.UUID,
		decision models.DecisionToCreate,
		scenarioIterationId string,
	) error
	EnqueueDecisionTaskMany(
		ctx context.Context,
		tx Transaction,
		organizationId uuid.UUID,
		decisions []models.DecisionToCreate,
		scenarioIterationId string,
	) error
	EnqueueScheduledExecStatusTask(
		ctx context.Context,
		tx Transaction,
		organizationId uuid.UUID,
		scheduledExecutionId string,
	) error
	EnqueueCreateIndexTask(
		ctx context.Context,
		organizationId uuid.UUID,
		indices []models.ConcreteIndex,
	) error
	EnqueueMatchEnrichmentTask(
		ctx context.Context,
		tx Transaction,
		organizationId uuid.UUID,
		screeningId string,
	) error
	EnqueueCaseReviewTask(
		ctx context.Context,
		tx Transaction,
		organizationId uuid.UUID,
		caseId uuid.UUID,
		aiCaseReviewId uuid.UUID,
	) error
	EnqueueAutoAssignmentTask(
		ctx context.Context,
		tx Transaction,
		orgId uuid.UUID,
		inboxId uuid.UUID,
	) error
	EnqueueDecisionWorkflowTask(
		ctx context.Context,
		tx Transaction,
		orgId uuid.UUID,
		decisionId string,
	) error
	EnqueueSendBillingEventTask(
		ctx context.Context,
		event models.BillingEvent,
	) error
	EnqueueContinuousScreeningDoScreeningTaskMany(
		ctx context.Context,
		tx Transaction,
		orgId uuid.UUID,
		objectType string,
		enqueueObjectUpdateTasks []models.ContinuousScreeningEnqueueObjectUpdateTask,
		triggerType models.ContinuousScreeningTriggerType,
	) error
	EnqueueContinuousScreeningApplyDeltaFileTask(
		ctx context.Context,
		tx Transaction,
		orgId uuid.UUID,
		updateId uuid.UUID,
	) error
	EnqueueCsvIngestionTask(
		ctx context.Context,
		tx Transaction,
		organizationId uuid.UUID,
		uploadLogId string,
	) error
	EnqueueScheduledExecutionTask(
		ctx context.Context,
		tx Transaction,
		organizationId uuid.UUID,
		scheduledExecutionId string,
	) error
	EnqueueContinuousScreeningMatchEnrichmentTask(
		ctx context.Context,
		tx Transaction,
		organizationId uuid.UUID,
		continuousScreeningId uuid.UUID,
	) error
}

type riverRepository struct {
	client *river.Client[pgx.Tx]
}

func NewTaskQueueRepository(client *river.Client[pgx.Tx]) TaskQueueRepository {
	return riverRepository{client: client}
}

func (r riverRepository) EnqueueDecisionTask(
	ctx context.Context,
	tx Transaction,
	organizationId uuid.UUID,
	decision models.DecisionToCreate,
	scenarioIterationId string,
) error {
	res, err := r.client.InsertTx(ctx, tx.RawTx(), models.AsyncDecisionArgs{
		DecisionToCreateId:   decision.Id,
		ObjectId:             decision.ObjectId,
		ScheduledExecutionId: decision.ScheduledExecutionId,
		ScenarioIterationId:  scenarioIterationId,
	}, &river.InsertOpts{
		MaxAttempts: nbRetriesAsyncDecision,
		Priority:    priorityAsyncDecision,
		Queue:       organizationId.String(),
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	})
	if err != nil {
		return err
	}
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued decision task", "decision_id", decision.Id, "job_id", res.Job.ID)
	return nil
}

func (r riverRepository) EnqueueDecisionTaskMany(
	ctx context.Context,
	tx Transaction,
	organizationId uuid.UUID,
	decisions []models.DecisionToCreate,
	scenarioIterationId string,
) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "start enqueueing batch of decision tasks")
	t := time.Now()
	params := make([]river.InsertManyParams, len(decisions))
	for i, decision := range decisions {
		params[i] = river.InsertManyParams{
			Args: models.AsyncDecisionArgs{
				DecisionToCreateId:   decision.Id,
				ObjectId:             decision.ObjectId,
				ScheduledExecutionId: decision.ScheduledExecutionId,
				ScenarioIterationId:  scenarioIterationId,
			},
			InsertOpts: &river.InsertOpts{
				MaxAttempts: nbRetriesAsyncDecision,
				Priority:    priorityAsyncDecision,
				Queue:       organizationId.String(),
				UniqueOpts:  river.UniqueOpts{
					// ByArgs: true,
				},
			},
		}
	}

	pgtx := tx.RawTx()
	res, err := r.client.InsertManyFastTx(ctx, pgtx, params)
	if err != nil {
		return err
	}

	utils.LoggerFromContext(ctx).
		InfoContext(ctx, fmt.Sprintf("Enqueued %d decision tasks in %s", res, time.Since(t)))
	return nil
}

func (r riverRepository) EnqueueScheduledExecStatusTask(
	ctx context.Context,
	tx Transaction,
	organizationId uuid.UUID,
	scheduledExecutionId string,
) error {
	res, err := r.client.InsertTx(ctx, tx.RawTx(), models.ScheduledExecStatusSyncArgs{
		ScheduledExecutionId: scheduledExecutionId,
	}, &river.InsertOpts{
		MaxAttempts: nbRetriesScheduledExecStatus,
		Priority:    priorityScheduledExecStatus,
		Queue:       organizationId.String(),
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	})
	if err != nil {
		return err
	}
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued scheduled execution status update task", "job_id", res.Job.ID)
	return nil
}

func (r riverRepository) EnqueueCreateIndexTask(
	ctx context.Context,
	organizationId uuid.UUID,
	indices []models.ConcreteIndex,
) error {
	_, err := r.client.Insert(
		ctx,
		models.IndexCreationArgs{
			OrgId:   organizationId,
			Indices: indices,
		},
		&river.InsertOpts{
			Queue: organizationId.String(),
		})
	if err != nil {
		return err
	}

	return nil
}

func (r riverRepository) EnqueueMatchEnrichmentTask(
	ctx context.Context,
	tx Transaction,
	organizationId uuid.UUID,
	screeningId string,
) error {
	res, err := r.client.InsertTx(
		ctx,
		tx.RawTx(),
		models.MatchEnrichmentArgs{
			OrgId:       organizationId,
			ScreeningId: screeningId,
		},
		&river.InsertOpts{
			Queue: organizationId.String(),
		},
	)
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued scheduled execution match enrichment task", "job_id", res.Job.ID)

	return nil
}

func (r riverRepository) EnqueueCaseReviewTask(
	ctx context.Context,
	tx Transaction,
	organizationId uuid.UUID,
	caseId uuid.UUID,
	aiCaseReviewId uuid.UUID,
) error {
	res, err := r.client.InsertTx(
		ctx,
		tx.RawTx(),
		models.CaseReviewArgs{
			CaseId:         caseId,
			AiCaseReviewId: aiCaseReviewId,
		},
		&river.InsertOpts{
			Queue: organizationId.String(),
		},
	)
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued case review task", "job_id", res.Job.ID)
	return nil
}

func (r riverRepository) EnqueueAutoAssignmentTask(
	ctx context.Context,
	tx Transaction,
	orgId uuid.UUID,
	inboxId uuid.UUID,
) error {
	res, err := r.client.InsertTx(
		ctx,
		tx.RawTx(),
		models.AutoAssignmentArgs{
			OrgId:   orgId,
			InboxId: inboxId,
		},
		&river.InsertOpts{
			Queue:       orgId.String(),
			ScheduledAt: time.Now().Add(time.Minute),
			UniqueOpts: river.UniqueOpts{
				ByQueue:  true,
				ByPeriod: 2 * time.Minute,
			},
		})
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued scheduled execution for case auto-assignment", "job_id", res.Job.ID)

	return nil
}

func (r riverRepository) EnqueueDecisionWorkflowTask(
	ctx context.Context,
	tx Transaction,
	orgId uuid.UUID,
	decisionId string,
) error {
	res, err := r.client.InsertTx(
		ctx,
		tx.RawTx(),
		models.DecisionWorkflowArgs{
			DecisionId: decisionId,
		},
		&river.InsertOpts{
			Queue: orgId.String(),
		},
	)
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued decision workflow task", "job_id", res.Job.ID)
	return nil
}

func (r riverRepository) EnqueueSendBillingEventTask(
	ctx context.Context,
	event models.BillingEvent,
) error {
	logger := utils.LoggerFromContext(ctx)

	res, err := r.client.Insert(
		ctx,
		models.SendBillingEventArgs{
			Event: event,
		},
		&river.InsertOpts{
			Queue: models.BILLING_QUEUE_NAME,
		},
	)
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Enqueued send billing event task", "job_id", res.Job.ID)
	return nil
}

func (r riverRepository) EnqueueContinuousScreeningDoScreeningTaskMany(
	ctx context.Context,
	tx Transaction,
	orgId uuid.UUID,
	objectType string,
	enqueueObjectUpdateTasks []models.ContinuousScreeningEnqueueObjectUpdateTask,
	triggerType models.ContinuousScreeningTriggerType,
) error {
	params := make([]river.InsertManyParams, len(enqueueObjectUpdateTasks))
	for i, enqueueObjectUpdateTask := range enqueueObjectUpdateTasks {
		params[i] = river.InsertManyParams{
			Args: models.ContinuousScreeningDoScreeningArgs{
				ObjectType:         objectType,
				OrgId:              orgId,
				TriggerType:        triggerType,
				MonitoringId:       enqueueObjectUpdateTask.MonitoringId,
				PreviousInternalId: enqueueObjectUpdateTask.PreviousInternalId,
				NewInternalId:      enqueueObjectUpdateTask.NewInternalId,
			},
			InsertOpts: &river.InsertOpts{
				Queue:    orgId.String(),
				Priority: 4, // Low priority to avoid blocking other tasks
			},
		}
	}

	res, err := r.client.InsertManyFastTx(ctx, tx.RawTx(), params)
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued continuous screening do screening tasks", "nb_tasks", res)
	return nil
}

func (r riverRepository) EnqueueContinuousScreeningApplyDeltaFileTask(
	ctx context.Context,
	tx Transaction,
	orgId uuid.UUID,
	updateId uuid.UUID,
) error {
	res, err := r.client.InsertTx(ctx, tx.RawTx(), models.ContinuousScreeningApplyDeltaFileArgs{
		OrgId:    orgId,
		UpdateId: updateId,
	}, &river.InsertOpts{
		Queue:    orgId.String(),
		Priority: 4, // Low priority to avoid blocking other tasks
	})
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued continuous screening process delta file task", "job_id", res.Job.ID)
	return nil
}

func (r riverRepository) EnqueueCsvIngestionTask(
	ctx context.Context,
	tx Transaction,
	organizationId uuid.UUID,
	uploadLogId string,
) error {
	res, err := r.client.InsertTx(ctx, tx.RawTx(), models.CsvIngestionArgs{
		UploadLogId: uploadLogId,
	}, &river.InsertOpts{
		Queue: organizationId.String(),
	})
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued CSV ingestion task", "upload_log_id", uploadLogId, "job_id", res.Job.ID)
	return nil
}

func (r riverRepository) EnqueueScheduledExecutionTask(
	ctx context.Context,
	tx Transaction,
	organizationId uuid.UUID,
	scheduledExecutionId string,
) error {
	res, err := r.client.InsertTx(ctx, tx.RawTx(), models.ScheduledExecutionArgs{
		ScheduledExecutionId: scheduledExecutionId,
	}, &river.InsertOpts{
		Queue: organizationId.String(),
	})
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued scheduled execution task",
		"scheduled_execution_id", scheduledExecutionId, "job_id", res.Job.ID)
	return nil
}

func (r riverRepository) EnqueueContinuousScreeningMatchEnrichmentTask(
	ctx context.Context,
	tx Transaction,
	organizationId uuid.UUID,
	continuousScreeningId uuid.UUID,
) error {
	res, err := r.client.InsertTx(
		ctx,
		tx.RawTx(),
		models.ContinuousScreeningMatchEnrichmentArgs{
			ContinuousScreeningId: continuousScreeningId,
		},
		&river.InsertOpts{
			Queue: organizationId.String(),
		},
	)
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Enqueued continuous screening match enrichment task", "job_id", res.Job.ID)

	return nil
}
