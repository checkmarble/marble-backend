package worker_jobs

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

const (
	CLIENT_DATA_PURGE_INTERVAL = time.Hour
)

func NewClientDataPurgeJob(orgId uuid.UUID) *river.PeriodicJob {
	return NewPeriodicJob(
		river.PeriodicInterval(CLIENT_DATA_PURGE_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ClientDataPurgeArgs{OrgId: orgId},
				&river.InsertOpts{
					Queue:    orgId.String(),
					Priority: 4,
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: CLIENT_DATA_PURGE_INTERVAL,
					},
				}
		},
	)
}

type ClientDataPurgeWorker struct {
	river.WorkerDefaults[models.ClientDataPurgeArgs]

	executorFactory           executor_factory.ExecutorFactory
	dataModelRepository       repositories.DataModelRepository
	clientDataPurgeRepository repositories.ClientDataPurgeRepository
}

func NewClientDataPurgeWorker(
	executorFactory executor_factory.ExecutorFactory,
	dataModelRepository repositories.DataModelRepository,
	clientDataPurgeRepository repositories.ClientDataPurgeRepository,
) *ClientDataPurgeWorker {
	return &ClientDataPurgeWorker{
		executorFactory:           executorFactory,
		dataModelRepository:       dataModelRepository,
		clientDataPurgeRepository: clientDataPurgeRepository,
	}
}

func (w *ClientDataPurgeWorker) Timeout(job *river.Job[models.ClientDataPurgeArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *ClientDataPurgeWorker) Work(ctx context.Context, job *river.Job[models.ClientDataPurgeArgs]) error {
	exec := w.executorFactory.NewExecutor()
	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	dataModel, err := w.dataModelRepository.GetDataModel(ctx, exec, job.Args.OrgId, false, false)
	if err != nil {
		return err
	}

	logger := utils.LoggerFromContext(ctx)
	logger.Info("running client data purge job")

	for _, table := range dataModel.Tables {
		if !table.Lifecycle.Enabled {
			continue
		}

		if table.Lifecycle.DeleteActiveRowsAfter != nil {
			gate := time.Now().Add(-table.Lifecycle.DeleteActiveRowsAfter.ToTimeDuration())

			for {
				rows, err := w.clientDataPurgeRepository.DeleteActiveRowsBefore(ctx, clientDbExec, table.Name, gate)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						// Normal operations, we want to limit the time spent here, we can be canceled
						return nil
					}

					return err
				}

				if rows == 0 {
					break
				}

				logger.DebugContext(ctx, "deleted live rows for being too old", "rows", rows)
			}
		}

		if table.Lifecycle.DeleteStaleRowsAfter != nil {
			gate := time.Now().Add(-table.Lifecycle.DeleteStaleRowsAfter.ToTimeDuration())

			for {
				rows, err := w.clientDataPurgeRepository.DeleteStaleRowsBefore(ctx, clientDbExec, table.Name, gate)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return nil
					}

					return err
				}

				if rows == 0 {
					break
				}

				logger.DebugContext(ctx, "deleted tombstone rows for being too old", "rows", rows)
			}
		}
	}

	return nil
}
