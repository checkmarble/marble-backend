package usecases

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scheduled_execution"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

const (
	numberWorkersPerQueue = 5
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

		if err = w.removeQueuesFromMissingOrgs(ctx, orgs); err != nil {
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

	resp, err := w.riverClient.QueueList(ctx, river.NewQueueListParams().First(10000))
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

			w.riverClient.PeriodicJobs().Add(scheduled_execution.NewIndexCleanupPeriodicJob(orgId))
			w.riverClient.PeriodicJobs().Add(scheduled_execution.NewTestRunSummaryPeriodicJob(orgId))
		}
	}

	return nil
}

func (w *TaskQueueWorker) removeQueuesFromMissingOrgs(ctx context.Context,
	orgs []models.Organization,
) error {
	logger := utils.LoggerFromContext(ctx)

	orgMap := make(map[string]struct{})
	for _, org := range orgs {
		orgMap[org.Id] = struct{}{}
	}

	runningQueues, err := w.riverClient.QueueList(ctx, river.NewQueueListParams().First(10000))
	if err != nil {
		return err
	}

	for _, q := range runningQueues.Queues {
		if q.PausedAt != nil {
			continue
		}

		if _, ok := orgMap[q.Name]; !ok {
			logger.InfoContext(ctx, fmt.Sprintf("pausing queue for missing organization `%s`", q.Name))

			if err := w.riverClient.QueuePause(ctx, q.Name, nil); err != nil {
				logger.ErrorContext(ctx, fmt.Sprintf("could not pause queue for deleted organization: %s", err.Error()))
			}
		}
	}

	return nil
}

func QueuesFromOrgs(ctx context.Context, orgsRepo repositories.OrganizationRepository,
	execGetter repositories.ExecutorGetter, enableOffloading bool,
) (queues map[string]river.QueueConfig, periodics []*river.PeriodicJob, err error) {
	exec_fac := executor_factory.NewDbExecutorFactory(orgsRepo, execGetter)
	orgs, err := orgsRepo.AllOrganizations(ctx, exec_fac.NewExecutor())
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return nil, nil, err
	}

	queues = make(map[string]river.QueueConfig, len(orgs))
	periodics = make([]*river.PeriodicJob, 0, len(orgs)*2)

	for _, org := range orgs {
		periodics = append(periodics, []*river.PeriodicJob{
			scheduled_execution.NewIndexCleanupPeriodicJob(org.Id),
			scheduled_execution.NewTestRunSummaryPeriodicJob(org.Id),
		}...)

		if enableOffloading {
			periodics = append(periodics, scheduled_execution.NewOffloadingPeriodicJob(org.Id))
		}

		queues[org.Id] = river.QueueConfig{
			MaxWorkers: numberWorkersPerQueue,
		}
	}

	return queues, periodics, nil
}
