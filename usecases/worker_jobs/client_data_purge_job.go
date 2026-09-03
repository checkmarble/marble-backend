package worker_jobs

import (
	"context"
	"fmt"
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
			for {
				// We check retention settings again before each iteration, so we can stop deleting rows if the setting is disabled.
				lifecycle, err := w.getTableRetentionSettings(ctx, exec, job.Args.OrgId, table.Name)
				if err != nil {
					return errors.Wrap(err, "could not recheck retention settings")
				}

				if !lifecycle.Enabled || lifecycle.DeleteActiveRowsAfter == nil {
					logger.InfoContext(ctx, "lifecycle management was disabled during running purge iteration",
						"table", table)
					break
				}

				gate := time.Now().Add(-lifecycle.DeleteActiveRowsAfter.ToTimeDuration())

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

				logger.DebugContext(ctx, "deleted live rows for being too old",
					"table", table,
					"rows", rows)
			}
		}

		if table.Lifecycle.DeleteStaleRowsAfter != nil {
			for {
				// We check retention settings again before each iteration, so we can stop deleting rows if the setting is disabled.
				lifecycle, err := w.getTableRetentionSettings(ctx, exec, job.Args.OrgId, table.Name)
				if err != nil {
					return errors.Wrap(err, "could not recheck retention settings")
				}

				if !lifecycle.Enabled || lifecycle.DeleteStaleRowsAfter == nil {
					logger.InfoContext(ctx, "lifecycle management was disabled during running purge iteration",
						"table", table)
					break
				}

				gate := time.Now().Add(-lifecycle.DeleteStaleRowsAfter.ToTimeDuration())

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

				logger.DebugContext(ctx, "deleted tombstone rows for being too old",
					"table", table,
					"rows", rows)
			}
		}
	}

	return nil
}

func (w *ClientDataPurgeWorker) getTableRetentionSettings(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, tableName string) (models.TableLifecycle, error) {
	dataModel, err := w.dataModelRepository.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return models.TableLifecycle{}, err
	}

	table, ok := dataModel.Tables[tableName]
	if !ok {
		return models.TableLifecycle{}, fmt.Errorf("unknown table %s", tableName)
	}

	return table.Lifecycle, nil
}
