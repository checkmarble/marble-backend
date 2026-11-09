package worker_jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
)

type lifecycleIndexRepository interface {
	ListAllIndexes(context.Context, repositories.Executor, ...models.IndexType) ([]models.ConcreteIndex, error)
	CreateIndexesConcurrently(context.Context, repositories.Executor, []models.ConcreteIndex) error
}

type LifecycleIndexProvisionWorker struct {
	river.WorkerDefaults[models.LifecycleIndexProvisionArgs]
	executorFactory     executor_factory.ExecutorFactory
	dataModelRepository repositories.DataModelRepository
	indexRepository     lifecycleIndexRepository
}

func NewLifecycleIndexProvisionWorker(factory executor_factory.ExecutorFactory, dataModel repositories.DataModelRepository, indexes lifecycleIndexRepository) *LifecycleIndexProvisionWorker {
	return &LifecycleIndexProvisionWorker{executorFactory: factory, dataModelRepository: dataModel, indexRepository: indexes}
}

func (w *LifecycleIndexProvisionWorker) Timeout(*river.Job[models.LifecycleIndexProvisionArgs]) time.Duration {
	return 4 * time.Hour
}

func (w *LifecycleIndexProvisionWorker) Work(ctx context.Context, job *river.Job[models.LifecycleIndexProvisionArgs]) (outErr error) {
	lockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 4*time.Hour+time.Minute)
	defer cancel()
	db, release, err := w.executorFactory.NewPinnedClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}
	defer release()
	unlock, acquired, err := repositories.GetTryAdvisoryLock(lockCtx, db, fmt.Sprintf("lifecycle-index:%s:%s", job.Args.OrgId, job.Args.TableID))
	if err != nil {
		return err
	}
	if !acquired {
		return river.JobSnooze(time.Second * 10)
	}
	defer func() {
		outErr = unlock()
    }()
	table, err := w.dataModelRepository.GetDataModelTable(ctx, w.executorFactory.NewExecutor(), job.Args.TableID)
	if err != nil {
		return errors.Wrap(err, "load lifecycle table")
	}
	if table.OrganizationID != job.Args.OrgId {
		return river.JobCancel(errors.New("lifecycle table organization does not match job"))
	}
	if table.Lifecycle == nil {
		return nil
	}
	existing, err := w.indexRepository.ListAllIndexes(ctx, db, models.IndexTypeLifecycleActive, models.IndexTypeLifecycleStale)
	if err != nil {
		return err
	}
	var missing []models.ConcreteIndex
	for _, required := range models.LifecycleIndexes(table.Name, *table.Lifecycle) {
		if !models.LifecycleIndexExists(required, existing) {
			for _, index := range existing {
				if index.Type == required.Type && index.Equal(required) && index.Status == models.IndexStatusPending {
					return river.JobSnooze(time.Second * 10)
				}
			}
			missing = append(missing, required)
		}
	}
	return w.indexRepository.CreateIndexesConcurrently(ctx, db, missing)
}
