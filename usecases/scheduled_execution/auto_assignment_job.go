package scheduled_execution

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/feature_access"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type autoAssignmentUsecase interface {
	RunAutoAssigner(ctx context.Context, orgId string, inboxId uuid.UUID) error
}

type AutoAssignmentWorker struct {
	river.WorkerDefaults[models.AutoAssignmentArgs]

	featureAccessReader   feature_access.FeatureAccessReader
	autoAssignmentUsecase autoAssignmentUsecase
}

func NewAutoAssignmentWorker(featureAccess feature_access.FeatureAccessReader, uc autoAssignmentUsecase) *AutoAssignmentWorker {
	return &AutoAssignmentWorker{
		featureAccessReader:   featureAccess,
		autoAssignmentUsecase: uc,
	}
}

func (w *AutoAssignmentWorker) Work(ctx context.Context, job *river.Job[models.AutoAssignmentArgs]) error {
	features, err := w.featureAccessReader.GetOrganizationFeatureAccess(ctx, job.Args.OrgId, nil)
	if err != nil {
		return errors.Wrap(err, "could not check feature access")
	}

	if !features.AutoAssignment.IsAllowed() {
		return nil
	}

	return w.autoAssignmentUsecase.RunAutoAssigner(ctx, job.Args.OrgId, job.Args.InboxId)
}
