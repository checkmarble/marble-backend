package continuous_screening

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const MaxDeltaTracksPerOrg = 1000

// Periodic job
func NewContinuousScreeningCreateFullDatasetJob(interval time.Duration) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ContinuousScreeningCreateFullDatasetArgs{}, &river.InsertOpts{
				Queue: models.CONTINUOUS_SCREENING_CREATE_FULL_DATASET_QUEUE_NAME,
				UniqueOpts: river.UniqueOpts{
					ByQueue:  true,
					ByPeriod: interval,
				},
			}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type createFullDatasetWorkerRepository interface {
	ListContinuousScreeningDeltaTracksPendingByOrg(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		limit uint64,
	) ([]models.ContinuousScreeningDeltaTrack, error)

	ListOrgsWithContinuousScreeningConfigs(
		ctx context.Context,
		exec repositories.Executor,
	) ([]uuid.UUID, error)
}

type CreateFullDatasetWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningCreateFullDatasetArgs]
	executorFactory executor_factory.ExecutorFactory

	repo createFullDatasetWorkerRepository
}

func NewCreateFullDatasetWorker(executorFactory executor_factory.ExecutorFactory,
	repo createFullDatasetWorkerRepository,
) *CreateFullDatasetWorker {
	return &CreateFullDatasetWorker{
		executorFactory: executorFactory,
		repo:            repo,
	}
}

func (w *CreateFullDatasetWorker) Timeout(job *river.Job[models.ContinuousScreeningCreateFullDatasetArgs]) time.Duration {
	// TODO: need to monitor the time it takes to create the full dataset
	return 1 * time.Hour
}

func (w *CreateFullDatasetWorker) Work(ctx context.Context,
	job *river.Job[models.ContinuousScreeningCreateFullDatasetArgs],
) error {
	exec := w.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Creating full dataset", "job", job)
	orgIdsWithConfigs, err := w.repo.ListOrgsWithContinuousScreeningConfigs(ctx, exec)
	if err != nil {
		return errors.Wrap(err, "failed to list orgs with continuous screening configs")
	}
	for _, orgId := range orgIdsWithConfigs {
		for {
			deltaTracks, err := w.repo.ListContinuousScreeningDeltaTracksPendingByOrg(
				ctx,
				exec,
				orgId,
				MaxDeltaTracksPerOrg,
			)
			if err != nil {
				return errors.Wrap(err, "failed to list continuous screening delta tracks pending by org")
			}
			if len(deltaTracks) == 0 {
				continue
			}
			// TODO: create the full dataset
			logger.DebugContext(ctx, "Creating full dataset", "orgId", orgId, "deltaTracks", deltaTracks)
		}
	}

	logger.DebugContext(ctx, "Successfully created full dataset")
	return nil
}
