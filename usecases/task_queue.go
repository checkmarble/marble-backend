package usecases

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

const (
	numberWorkersPerQueue = 4
)

type TaskQueueWorker struct {
	executorFactory executor_factory.ExecutorFactory
	orgRepository   repositories.OrganizationRepository
	riverClient     *river.Client[pgx.Tx]
	mu              *sync.Mutex
}

func NewTaskQueueWorker(
	executorFactory executor_factory.ExecutorFactory,
	orgRepository repositories.OrganizationRepository,
	riverClient *river.Client[pgx.Tx],
) *TaskQueueWorker {
	return &TaskQueueWorker{
		executorFactory: executorFactory,
		orgRepository:   orgRepository,
		riverClient:     riverClient,
		mu:              &sync.Mutex{},
	}
}

func (w *TaskQueueWorker) RefreshQueuesFromOrgIds(ctx context.Context) {
	logger := utils.LoggerFromContext(ctx)
	refreshOrgs := func() error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		orgs, err := w.orgRepository.AllOrganizations(ctx, w.executorFactory.NewExecutor())
		if err != nil {
			return err
		}
		queues := make(map[string]river.QueueConfig, len(orgs))
		for _, org := range orgs {
			queues[org.Id] = river.QueueConfig{
				MaxWorkers: numberWorkersPerQueue,
			}
		}

		err = w.addMissingQueues(ctx, queues)
		if err != nil {
			return err
		}
		return nil
	}

	for {
		time.Sleep(1 * time.Minute)
		err := retry.Do(refreshOrgs,
			retry.Attempts(3),
			retry.LastErrorOnly(true),
			retry.OnRetry(func(n uint, err error) {
				logger.WarnContext(ctx, "Error occurred while refreshing queue list from org ids in TaskQueueWorker, retry: "+err.Error())
			}),
		)
		if err != nil {
			panic(err)
		}
	}
}

func (w *TaskQueueWorker) addMissingQueues(ctx context.Context, queues map[string]river.QueueConfig) error {
	logger := utils.LoggerFromContext(ctx)
	w.mu.Lock()
	defer w.mu.Unlock()

	resp, err := w.riverClient.QueueList(ctx, nil)
	if err != nil {
		return err
	}
	existingQueues := resp.Queues
	existingQueuesAsMap := make(map[string]struct{}, len(existingQueues))
	for _, q := range existingQueues {
		existingQueuesAsMap[q.Name] = struct{}{}
	}

	for orgId, q := range queues {
		if _, ok := existingQueuesAsMap[orgId]; !ok {
			err := w.riverClient.Queues().Add(orgId, q)
			if err != nil {
				return err
			}
			logger.InfoContext(ctx, fmt.Sprintf("Added queue for organization %s to task queue worker", orgId))
		}
	}

	return nil
}

func QueuesFromOrgs(ctx context.Context, orgsRepo repositories.OrganizationRepository, execGetter repositories.ExecutorGetter,
) (queues map[string]river.QueueConfig, err error) {
	exec_fac := executor_factory.NewDbExecutorFactory(orgsRepo, execGetter)
	orgs, err := orgsRepo.AllOrganizations(ctx, exec_fac.NewExecutor())
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return nil, err
	}
	queues = make(map[string]river.QueueConfig, len(orgs))
	for _, org := range orgs {
		queues[org.Id] = river.QueueConfig{
			MaxWorkers: numberWorkersPerQueue,
		}
	}
	return queues, nil
}
