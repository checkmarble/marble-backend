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
		organizationId string,
		decision models.DecisionToCreate,
		scenarioIterationId string,
	) error
	EnqueueDecisionTaskMany(
		ctx context.Context,
		tx Transaction,
		organizationId string,
		decisions []models.DecisionToCreate,
		scenarioIterationId string,
	) error
	EnqueueScheduledExecStatusTask(
		ctx context.Context,
		tx Transaction,
		organizationId string,
		scheduledExecutionId string,
	) error
	EnqueueCreateIndexTask(
		ctx context.Context,
		organizationId string,
		indices []models.ConcreteIndex,
	) error
	EnqueueMatchEnrichmentTask(
		ctx context.Context,
		tx Transaction,
		organizationId string,
		screeningId string,
	) error
	EnqueueCaseReviewTask(
		ctx context.Context,
		tx Transaction,
		organizationId string,
		caseId string,
	) error
	EnqueueAutoAssignmentTask(
		ctx context.Context,
		tx Transaction,
		orgId string, inboxId uuid.UUID,
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
	organizationId string,
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
		Queue:       organizationId,
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
	organizationId string,
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
				Queue:       organizationId,
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
	organizationId string,
	scheduledExecutionId string,
) error {
	res, err := r.client.InsertTx(ctx, tx.RawTx(), models.ScheduledExecStatusSyncArgs{
		ScheduledExecutionId: scheduledExecutionId,
	}, &river.InsertOpts{
		MaxAttempts: nbRetriesScheduledExecStatus,
		Priority:    priorityScheduledExecStatus,
		Queue:       organizationId,
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
	organizationId string,
	indices []models.ConcreteIndex,
) error {
	_, err := r.client.Insert(
		ctx,
		models.IndexCreationArgs{
			OrgId:   organizationId,
			Indices: indices,
		},
		&river.InsertOpts{
			Queue: organizationId,
		})
	if err != nil {
		return err
	}

	return nil
}

func (r riverRepository) EnqueueMatchEnrichmentTask(
	ctx context.Context,
	tx Transaction,
	organizationId string,
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
			Queue: organizationId,
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
	organizationId string,
	caseId string,
) error {
	res, err := r.client.InsertTx(
		ctx,
		tx.RawTx(),
		models.CaseReviewArgs{
			CaseId: caseId,
		},
		&river.InsertOpts{
			Queue: organizationId,
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
	orgId string, inboxId uuid.UUID,
) error {
	res, err := r.client.InsertTx(
		ctx,
		tx.RawTx(),
		models.AutoAssignmentArgs{
			OrgId:   orgId,
			InboxId: inboxId,
		},
		&river.InsertOpts{
			Queue:       orgId,
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
