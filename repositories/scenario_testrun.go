package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type ScenarioTestRunRepository interface {
	CreateTestRun(
		ctx context.Context,
		tx Transaction,
		testrunId string,
		input models.ScenarioTestRunCreateDbInput,
	) error
	ListRunningTestRun(ctx context.Context, exec Executor, organizationId string) ([]models.ScenarioTestRun, error)
	ListTestRunsByScenarioID(ctx context.Context, exec Executor, scenarioID string) ([]models.ScenarioTestRun, error)
	GetTestRunByLiveVersionID(
		ctx context.Context,
		exec Executor,
		liveVersionID string,
	) (*models.ScenarioTestRun, error)
	UpdateTestRunStatus(ctx context.Context, exec Executor,
		scenarioIterationID string, status models.TestrunStatus,
	) error
	GetTestRunByID(ctx context.Context, exec Executor, testrunID string) (models.ScenarioTestRun, error)
}

func selectTestruns() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectScenarioTestRunColumns...).
		From(dbmodels.TABLE_SCENARIO_TESTRUN)
}

func (repo *MarbleDbRepository) CreateTestRun(
	ctx context.Context,
	tx Transaction,
	testrunID string,
	input models.ScenarioTestRunCreateDbInput,
) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}
	err := ExecBuilder(
		ctx,
		tx,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_SCENARIO_TESTRUN).
			Columns(
				"id",
				"scenario_iteration_id",
				"live_scenario_iteration_id",
				"created_at",
				"expires_at",
				"status",
			).
			Values(
				testrunID,
				input.PhantomIterationId,
				input.LiveScenarioId,
				time.Now(),
				input.EndDate,
				models.Pending.String(),
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateTestRunStatus(ctx context.Context, exec Executor, testRunId string, status models.TestrunStatus) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIO_TESTRUN).
		Set("status", status.String()).
		Where(squirrel.Eq{"id": testRunId})
	if status == models.Down {
		query = query.Set("expires_at", time.Now())
	}

	err := ExecBuilder(
		ctx,
		exec,
		query,
	)
	return err
}

func (repo *MarbleDbRepository) GetTestRunByLiveVersionID(
	ctx context.Context, exec Executor, liveVersionID string,
) (*models.ScenarioTestRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := selectTestruns().
		Where(squirrel.Eq{"live_scenario_iteration_id": liveVersionID}).
		OrderBy("created_at DESC")
	testruns, err := SqlToListOfModels(ctx, exec, query, dbmodels.AdaptScenarioTestrun)
	if err != nil {
		return nil, err
	}
	if len(testruns) == 0 {
		return nil, nil
	}
	return &testruns[0], nil
}

func (repo *MarbleDbRepository) ListRunningTestRun(
	ctx context.Context, exec Executor, organizationId string,
) ([]models.ScenarioTestRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(`
			tr.id, 
			tr.scenario_iteration_id,
			tr.live_scenario_iteration_id,
			tr.created_at, 
			tr.expires_at,
			tr.status,
			tr.summarized,
			tr.updated_at`).
		From(dbmodels.TABLE_SCENARIO_TESTRUN + " AS tr").
		Join(dbmodels.TABLE_SCENARIO_ITERATIONS + " AS scit ON scit.id = tr.scenario_iteration_id").
		Where(squirrel.And{
			squirrel.Eq{"tr.status": models.Up},
			squirrel.Eq{"scit.org_id": organizationId},
		}).
		OrderBy("created_at DESC")
	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptScenarioTestrun,
	)
}

func (repo *MarbleDbRepository) ListTestRunsByScenarioID(ctx context.Context,
	exec Executor, scenarioID string,
) ([]models.ScenarioTestRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select("tr.id, tr.scenario_iteration_id, tr.live_scenario_iteration_id, tr.created_at, tr.expires_at, tr.status, tr.summarized, tr.updated_at, scit.org_id, scit.scenario_id").
		From(dbmodels.TABLE_SCENARIO_TESTRUN + " AS tr").
		Join(dbmodels.TABLE_SCENARIO_ITERATIONS + " AS scit ON scit.id = tr.scenario_iteration_id").
		Join(dbmodels.TABLE_SCENARIOS + " AS sc ON sc.id = scit.scenario_id").
		Where(squirrel.Eq{"sc.id": scenarioID}).
		OrderBy("tr.created_at DESC")
	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptScenarioTestrunWithInfo,
	)
}

func (repo *MarbleDbRepository) GetTestRunByID(ctx context.Context, exec Executor, testrunID string) (models.ScenarioTestRun, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioTestRun{}, err
	}
	query := NewQueryBuilder().
		Select(`tr.id,
			tr.scenario_iteration_id,
			tr.live_scenario_iteration_id,
			tr.created_at,
			tr.expires_at,
			tr.status,
			tr.summarized,
			tr.updated_at,
			scit.org_id,
			scit.scenario_id`).
		From(dbmodels.TABLE_SCENARIO_TESTRUN + " AS tr").
		Join(dbmodels.TABLE_SCENARIO_ITERATIONS + " AS scit ON scit.id = tr.scenario_iteration_id").
		Where(squirrel.Eq{"tr.id": testrunID})

	return SqlToModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptScenarioTestrunWithInfo,
	)
}

func (repo *MarbleDbRepository) GetRecentTestRunForOrg(ctx context.Context, exec Executor,
	orgId string,
) ([]models.ScenarioTestRunWithSummary, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(columnsNames("str", dbmodels.SelectScenarioTestRunColumns)...).
		Columns(columnsNames("si", []string{"org_id", "scenario_id"})...).
		Column(fmt.Sprintf("ARRAY_AGG(ROW(%s)) filter (where strs.id is not null) AS summaries",
			strings.Join(columnsNames("strs", dbmodels.SelectScenarioTestRunSummariesColumns), ","))).
		From(dbmodels.TABLE_SCENARIO_TESTRUN+" as str").
		Join(dbmodels.TABLE_SCENARIO_ITERATIONS+" as si on si.id = str.scenario_iteration_id").
		LeftJoin("scenario_test_run_summaries as strs on strs.test_run_id = str.id").
		Where(squirrel.Eq{"si.org_id": orgId, "str.summarized": false}).
		Where(squirrel.Or{
			squirrel.Eq{"strs.watermark": nil},
			squirrel.And{
				squirrel.Expr("strs.watermark < str.expires_at"),
				squirrel.Expr("strs.watermark < now()"),
			},
		}).
		GroupBy("str.id", "si.org_id", "si.scenario_id")

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScenarioTestrunWithSummary)
}

func (repo *MarbleDbRepository) SaveTestRunSummary(ctx context.Context, exec Executor,
	testRunId string, stat models.RuleExecutionStat, newWatermark time.Time,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCENARIO_TESTRUN_SUMMARIES+" as orig").
		Columns("test_run_id", "version", "rule_stable_id", "rule_name", "watermark", "outcome", "total").
		Values(testRunId, stat.Version, stat.StableRuleId, stat.Name, newWatermark, stat.Outcome, stat.Total).
		Suffix(`
			on conflict (test_run_id, version, rule_stable_id, outcome) do update
			set
				watermark = ?,
				total = orig.total + EXCLUDED.total
		`, newWatermark)

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) SaveTestRunDecisionSummary(ctx context.Context, exec Executor,
	testRunId string, stat models.DecisionsByVersionByOutcome, newWatermark time.Time,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCENARIO_TESTRUN_SUMMARIES+" as orig").
		Columns("test_run_id", "version", "watermark", "outcome", "total").
		Values(testRunId, stat.Version, newWatermark, stat.Outcome, stat.Count).
		Suffix(`
			on conflict (test_run_id, version, rule_stable_id, outcome) do update
			set
				watermark = ?,
				total = orig.total + EXCLUDED.total
		`, newWatermark)

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) BumpDecisionSummaryWatermark(ctx context.Context, exec Executor,
	testRunId string, newWatermark time.Time,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIO_TESTRUN_SUMMARIES).
		Where(squirrel.Eq{"test_run_id": testRunId}).
		Set("watermark", newWatermark)

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) ReadLatestUpdatedAt(ctx context.Context, exec Executor, testRunId string) (time.Time, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return time.Time{}, err
	}

	sql := NewQueryBuilder().
		Select("updated_at").
		From(dbmodels.TABLE_SCENARIO_TESTRUN).
		Where(squirrel.Eq{"id": testRunId})

	query, args, err := sql.ToSql()
	if err != nil {
		return time.Time{}, err
	}

	row := exec.QueryRow(ctx, query, args...)

	var updatedAt time.Time

	if err := row.Scan(&updatedAt); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (repo *MarbleDbRepository) TouchLatestUpdatedAt(ctx context.Context, exec Executor, testRunId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIO_TESTRUN).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": testRunId})

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) SetTestRunAsSummarized(ctx context.Context, exec Executor, testRunId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCENARIO_TESTRUN).
		Set("summarized", true).
		Where(squirrel.Eq{"id": testRunId})

	return ExecBuilder(ctx, exec, sql)
}
