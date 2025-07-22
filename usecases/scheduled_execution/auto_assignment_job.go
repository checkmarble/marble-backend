package scheduled_execution

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/riverqueue/river"
)

type autoAssignmentUsecase interface {
	RunAutoAssigner(ctx context.Context, orgId, inboxId string) error
}

type AutoAssignmentWorker struct {
	river.WorkerDefaults[models.AutoAssignmentArgs]

	autoAssignmentUsecase autoAssignmentUsecase
}

func NewAutoAssignmentWorker(uc autoAssignmentUsecase) *AutoAssignmentWorker {
	return &AutoAssignmentWorker{
		autoAssignmentUsecase: uc,
	}
}

func (w *AutoAssignmentWorker) Work(ctx context.Context, job *river.Job[models.AutoAssignmentArgs]) error {
	return w.autoAssignmentUsecase.RunAutoAssigner(ctx, job.Args.OrgId, job.Args.InboxId)
}
