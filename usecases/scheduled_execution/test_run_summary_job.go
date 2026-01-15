package scheduled_execution

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	// The summary is not idempotent, so we cannot afford to have two processes running at the same time for the same organization.
	// Be mindful to not set the timeout greater (or even close) to the interval, to prevent that.
	TEST_RUN_SUMMARY_TIMEOUT         = 2 * time.Minute
	TEST_RUN_SUMMARY_WORKER_INTERVAL = 5 * time.Minute
	TEST_RUN_SUMMARY_WINDOW          = 6 * time.Hour
)

type RulesRepository interface {
	GetRecentTestRunForOrg(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) (
		[]models.ScenarioTestRunWithSummary, error)
	RulesExecutionStats(
		ctx context.Context,
		exec repositories.Transaction,
		organizationId uuid.UUID,
		iterationId string,
		begin, end time.Time,
	) ([]models.RuleExecutionStat, error)
	PhanomRulesExecutionStats(
		ctx context.Context,
		exec repositories.Transaction,
		organizationId uuid.UUID,
		testRunId string,
		begin, end time.Time,
	) ([]models.RuleExecutionStat, error)
	ScreeningExecutionStats(
		ctx context.Context,
		exec repositories.Executor,
		organizationId uuid.UUID,
		iterationId string,
		begin, end time.Time,
		base string, // "decisions" or "phantom_decisions"
	) ([]models.RuleExecutionStat, error)
	DecisionsByOutcomeAndScore(
		ctx context.Context,
		exec repositories.Executor,
		organizationId uuid.UUID,
		scenarioId string,
		scenarioLiveIterationId string,
		begin, end time.Time,
	) ([]models.DecisionsByVersionByOutcome, error)
	SaveTestRunDecisionSummary(ctx context.Context, exec repositories.Executor, testRunId string,
		stat models.DecisionsByVersionByOutcome, newWatermark time.Time) error
	SaveTestRunSummary(ctx context.Context, exec repositories.Executor,
		testRunId string, stat models.RuleExecutionStat, newWatermark time.Time,
	) error
	BumpDecisionSummaryWatermark(ctx context.Context, exec repositories.Executor,
		testRunId string, newWatermark time.Time,
	) error
	SetTestRunAsSummarized(ctx context.Context, exec repositories.Executor, testRunId string) error
	ReadLatestUpdatedAt(ctx context.Context, exec repositories.Executor, testRunId string) (time.Time, error)
	TouchLatestUpdatedAt(ctx context.Context, exec repositories.Executor, testRunId string) error
}

func NewTestRunSummaryPeriodicJob(orgId uuid.UUID) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(TEST_RUN_SUMMARY_WORKER_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.TestRunSummaryArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId.String(),
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: TEST_RUN_SUMMARY_WORKER_INTERVAL,
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

func (w *TestRunSummaryWorker) Timeout(job *river.Job[models.TestRunSummaryArgs]) time.Duration {
	return TEST_RUN_SUMMARY_TIMEOUT
}

func (w *TestRunSummaryWorker) Work(ctx context.Context, job *river.Job[models.TestRunSummaryArgs]) error {
	testRuns, err := w.repository.GetRecentTestRunForOrg(ctx, w.executor_factory.NewExecutor(), job.Args.OrgId)
	if err != nil {
		return err
	}

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

		windowBound := then.Add(TEST_RUN_SUMMARY_WINDOW)

		if windowBound.After(time.Now()) {
			windowBound = time.Now()
		}

		err := w.transaction_factory.Transaction(ctx, func(tx repositories.Transaction) error {
			// Warning: this method is implemented to return at least one count object with 0 count by default, because the watermark on it is
			// needed for the summary calculation.
			// This logic is not implemented in all the subsequent repositories methods, but only because they are expected to be called together
			// successively.
			// TL:DR: it the job runs and there are no decisions, at least one summary must be created to advance the watermark.
			decisionStats, err := w.repository.DecisionsByOutcomeAndScore(ctx, tx,
				job.Args.OrgId, testRun.ScenarioId, testRun.ScenarioLiveIterationId, then, windowBound)
			if err != nil {
				return err
			}

			liveStats, err := w.repository.RulesExecutionStats(ctx, tx, job.Args.OrgId,
				testRun.ScenarioLiveIterationId, then, windowBound)
			if err != nil {
				return err
			}

			phantomStats, err := w.repository.PhanomRulesExecutionStats(ctx, tx,
				job.Args.OrgId, testRun.ScenarioIterationId, then, windowBound)
			if err != nil {
				return err
			}

			liveScreeningStats, err := w.repository.ScreeningExecutionStats(
				ctx, tx, job.Args.OrgId, testRun.ScenarioLiveIterationId, then, windowBound, "decisions")
			if err != nil {
				return err
			}

			phantomScreeningStats, err := w.repository.ScreeningExecutionStats(
				ctx, tx, job.Args.OrgId, testRun.ScenarioIterationId, then, windowBound, "phantom_decisions")
			if err != nil {
				return err
			}

			for _, stat := range decisionStats {
				if err := w.repository.SaveTestRunDecisionSummary(ctx, tx,
					testRun.Id, stat, windowBound); err != nil {
					return err
				}
			}

			savedNewData := false

			for _, results := range [][]models.RuleExecutionStat{
				liveStats, phantomStats, liveScreeningStats, phantomScreeningStats,
			} {
				for _, stat := range results {
					savedNewData = true

					if err := w.repository.SaveTestRunSummary(ctx, tx,
						testRun.Id, stat, windowBound); err != nil {
						return err
					}
				}
			}

			// Once all summaries have been written, update the watermark on all of them, even those that have not been updated in this run.
			if err := w.repository.BumpDecisionSummaryWatermark(ctx, tx, testRun.Id, windowBound); err != nil {
				return err
			}

			if testRun.Status == models.Down || windowBound.After(testRun.ExpiresAt) {
				if err := w.repository.SetTestRunAsSummarized(ctx, tx, testRun.Id); err != nil {
					return err
				}
			}

			newUpdatedAt, err := w.repository.ReadLatestUpdatedAt(ctx, tx, testRun.Id)
			if err != nil {
				return err
			}

			if !newUpdatedAt.Equal(testRun.UpdatedAt) {
				utils.LoggerFromContext(ctx).WarnContext(ctx,
					"test run summary job rolled back because of detected concurrent access, rolling back")
				return fmt.Errorf("outdated concurrency key in test run summary job, rolling back")
			}

			if savedNewData {
				if err := w.repository.TouchLatestUpdatedAt(ctx, tx, testRun.Id); err != nil {
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
