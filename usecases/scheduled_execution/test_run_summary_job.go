package scheduled_execution

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/riverqueue/river"
)

const TEST_RUN_SUMMARY_WORKER_INTERVAL = 5 * time.Minute

type RulesRepository interface {
	GetRecentTestRunForOrg(ctx context.Context, exec repositories.Executor, orgId string) (
		[]models.ScenarioTestRunWithSummary, error)
	RulesExecutionStats(
		ctx context.Context,
		exec repositories.Transaction,
		organizationId string,
		iterationId string,
		begin, end time.Time,
	) ([]models.RuleExecutionStat, error)
	PhanomRulesExecutionStats(
		ctx context.Context,
		exec repositories.Transaction,
		organizationId string,
		testRunId string,
		begin, end time.Time,
	) ([]models.RuleExecutionStat, error)
	SaveTestRunSummary(ctx context.Context, exec repositories.Executor,
		testRunId string, stat models.RuleExecutionStat, newWatermark time.Time,
	) error
	SetTestRunAsSummarized(ctx context.Context, exec repositories.Executor, testRunId string) error
}

func NewTestRunSummaryPeriodicJob(orgId string) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(TEST_RUN_SUMMARY_WORKER_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.TestRunSummaryArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId,
					UniqueOpts: river.UniqueOpts{
						ByArgs: true,
					},
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type TestRunSummaryWorker struct {
	river.WorkerDefaults[models.TestRunSummaryArgs]

	executor_factory    executor_factory.ExecutorFactory
	transaction_factory executor_factory.TransactionFactory
	repository          RulesRepository
}

func NewTestRunSummaryWorker(
	executor_factory executor_factory.ExecutorFactory,
	transaction_factory executor_factory.TransactionFactory,
	repository RulesRepository,
) TestRunSummaryWorker {
	return TestRunSummaryWorker{
		executor_factory:    executor_factory,
		transaction_factory: transaction_factory,
		repository:          repository,
	}
}

func (w *TestRunSummaryWorker) Work(ctx context.Context, job *river.Job[models.TestRunSummaryArgs]) error {
	testRuns, err := w.repository.GetRecentTestRunForOrg(ctx, w.executor_factory.NewExecutor(), job.Args.OrgId)
	if err != nil {
		return err
	}

	now := time.Now()

	for _, testRun := range testRuns {
		then := testRun.CreatedAt

		var earliestWatermark *time.Time

		for _, s := range testRun.Summary {
			if earliestWatermark == nil || s.Watermark.Before(*earliestWatermark) {
				earliestWatermark = &s.Watermark
			}
		}
		if earliestWatermark != nil {
			then = *earliestWatermark
		}

		err := w.transaction_factory.Transaction(ctx, func(tx repositories.Transaction) error {
			liveStats, err := w.repository.RulesExecutionStats(ctx, tx, job.Args.OrgId,
				testRun.ScenarioLiveIterationId, then, now)
			if err != nil {
				return err
			}

			phantomStats, err := w.repository.PhanomRulesExecutionStats(ctx, tx,
				job.Args.OrgId, testRun.ScenarioIterationId, then, now)
			if err != nil {
				return err
			}

			for _, stat := range liveStats {
				if err := w.repository.SaveTestRunSummary(ctx, tx, testRun.Id, stat, now); err != nil {
					return err
				}
			}
			for _, stat := range phantomStats {
				if err := w.repository.SaveTestRunSummary(ctx, tx, testRun.Id, stat, now); err != nil {
					return err
				}
			}

			if testRun.Status == models.Down {
				if err := w.repository.SetTestRunAsSummarized(ctx, tx, testRun.Id); err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}
