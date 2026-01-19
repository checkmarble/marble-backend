package usecases

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/continuous_screening"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scheduled_execution"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

const (
	numberWorkersPerQueue = 5
)

type TaskQueueWorker struct {
	executorFactory executor_factory.ExecutorFactory
	orgRepository   repositories.OrganizationRepository
	queueWhitelist  []string
	riverClient     *river.Client[pgx.Tx]
	mu              *sync.Mutex
}

func NewTaskQueueWorker(
	executorFactory executor_factory.ExecutorFactory,
	orgRepository repositories.OrganizationRepository,
	queueWhitelist []string,
	riverClient *river.Client[pgx.Tx],
) *TaskQueueWorker {
	return &TaskQueueWorker{
		executorFactory: executorFactory,
		orgRepository:   orgRepository,
		queueWhitelist:  queueWhitelist,
		riverClient:     riverClient,
		mu:              &sync.Mutex{},
	}
}

func (w *TaskQueueWorker) RefreshQueuesFromOrgIds(
	ctx context.Context,
	offloadingConfig infra.OffloadingConfig,
	analyticsConfig infra.AnalyticsConfig,
	csCreateFullDatasetInterval time.Duration,
) {
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
			queues[org.Id.String()] = river.QueueConfig{
				MaxWorkers: numberWorkersPerQueue,
			}
		}

		err = w.addMissingQueues(ctx, queues, offloadingConfig, analyticsConfig, csCreateFullDatasetInterval)
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

func (w *TaskQueueWorker) addMissingQueues(
	ctx context.Context,
	queues map[string]river.QueueConfig,
	offloadingConfig infra.OffloadingConfig,
	analyticsConfig infra.AnalyticsConfig,
	csCreateFullDatasetInterval time.Duration,
) error {
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
			orgIdUuid, err := uuid.Parse(orgId)
			if err != nil {
				return err
			}
			org, err := w.orgRepository.GetOrganizationById(ctx,
				w.executorFactory.NewExecutor(), orgIdUuid)
			if err != nil {
				return err
			}
			err = w.riverClient.Queues().Add(orgId, q)
			if err != nil {
				return err
			}
			logger.InfoContext(ctx, fmt.Sprintf("Added queue for organization %s to task queue worker", orgId))

			for _, p := range listOrgPeriodics(org, offloadingConfig, analyticsConfig, csCreateFullDatasetInterval) {
				w.riverClient.PeriodicJobs().Add(p)
			}
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
		orgMap[org.Id.String()] = struct{}{}
	}

	runningQueues, err := w.riverClient.QueueList(ctx, river.NewQueueListParams().First(10000))
	if err != nil {
		return err
	}

	for _, q := range runningQueues.Queues {
		if q.PausedAt != nil {
			continue
		}

		// Ignore whitelisted queues
		if slices.Contains(w.queueWhitelist, q.Name) {
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

func listOrgPeriodics(
	org models.Organization,
	offloadingConfig infra.OffloadingConfig,
	analyticsConfig infra.AnalyticsConfig,
	csCreateFullDatasetInterval time.Duration,
) []*river.PeriodicJob {
	periodics := []*river.PeriodicJob{
		scheduled_execution.NewIndexCleanupPeriodicJob(org.Id),
		scheduled_execution.NewIndexDeletionPeriodicJob(org.Id),
		scheduled_execution.NewTestRunSummaryPeriodicJob(org.Id),
		continuous_screening.NewContinuousScreeningCreateFullDatasetPeriodicJob(
			org.Id,
			// TODO: Configurable per Org
			csCreateFullDatasetInterval,
		),
		// Migrated from cron jobs
		scheduled_execution.NewScheduledScenarioPeriodicJob(org.Id),
		scheduled_execution.NewCsvIngestionPeriodicJob(org.Id),
		scheduled_execution.NewWebhookRetryPeriodicJob(org.Id),
	}
	if offloadingConfig.Enabled {
		// Undocumented debug setting to only enable offloading for a specific organization
		if onlyOffloadOrg := os.Getenv("OFFLOADING_ONLY_ORG"); onlyOffloadOrg == "" || onlyOffloadOrg == org.Id.String() {
			periodics = append(periodics, scheduled_execution.NewOffloadingPeriodicJob(org.Id, offloadingConfig.JobInterval))
		}
	}

	if analyticsConfig.Enabled {
		periodics = append(periodics, scheduled_execution.NewAnalyticsExportJob(org.Id, analyticsConfig.JobInterval))
	}

	return periodics
}

func QueuesFromOrgs(
	ctx context.Context,
	appName string,
	orgsRepo repositories.OrganizationRepository,
	execGetter repositories.ExecutorGetter,
	offloadingConfig infra.OffloadingConfig,
	analyticsConfig infra.AnalyticsConfig,
	csCreateFullDatasetInterval time.Duration,
) (queues map[string]river.QueueConfig, periodics []*river.PeriodicJob, err error) {
	exec_fac := executor_factory.NewDbExecutorFactory(appName, orgsRepo, execGetter)
	orgs, err := orgsRepo.AllOrganizations(ctx, exec_fac.NewExecutor())
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return nil, nil, err
	}

	queues = make(map[string]river.QueueConfig, len(orgs))

	for _, org := range orgs {
		periodics = append(periodics, listOrgPeriodics(org, offloadingConfig,
			analyticsConfig, csCreateFullDatasetInterval)...)

		queues[org.Id.String()] = river.QueueConfig{
			MaxWorkers: numberWorkersPerQueue,
		}
	}

	return queues, periodics, nil
}

func QueueMetrics() map[string]river.QueueConfig {
	queues := make(map[string]river.QueueConfig, 1)
	queues[models.METRICS_QUEUE_NAME] = river.QueueConfig{
		MaxWorkers: 1,
	}
	return queues
}

func QueueBilling() map[string]river.QueueConfig {
	queues := make(map[string]river.QueueConfig, 1)
	queues[models.BILLING_QUEUE_NAME] = river.QueueConfig{
		MaxWorkers: 1,
	}
	return queues
}

func QueueAnalyticsMerge() map[string]river.QueueConfig {
	queues := make(map[string]river.QueueConfig, 1)
	queues["analytics_merge"] = river.QueueConfig{
		MaxWorkers: 1,
	}
	return queues
}

func QueueContinuousScreeningDatasetUpdate() map[string]river.QueueConfig {
	queues := make(map[string]river.QueueConfig, 1)
	queues[models.CONTINUOUS_SCREENING_DATASET_UPDATE_QUEUE_NAME] = river.QueueConfig{
		MaxWorkers: 1,
	}
	return queues
}
