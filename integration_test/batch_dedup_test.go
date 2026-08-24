package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/analytics-go/v3"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// TestBatchExecutionDeduplication exercises the v2 (manifest-based) batch coordinator with
// Scenario.DeduplicateBatchObjects enabled: running the same scenario twice over the same
// ingested object must create a decision only on the first run, and must skip the object
// before evaluation (not just before decision creation) on the second run.
func TestBatchExecutionDeduplication(t *testing.T) {
	// The feature flag is read live from the environment on every check, and only the v2
	// coordinator honours the dedup setting.
	t.Setenv("ENABLE_BATCH_EXECUTION_V2", "all")

	ctx := context.Background()
	ctx = utils.StoreLoggerInContext(ctx, utils.NewLogger("text"))
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

	userCreds, _, inboxId := setupOrgAndCreds(ctx, t, "test org with batch dedup")
	organizationId := userCreds.OrganizationId
	usecasesWithCreds := generateUsecaseWithCreds(testUsecases, userCreds)

	rules := getRulesForBatchTest()
	scenarioId, scenarioIterationId := setupScenarioAndPublish(ctx, t, usecasesWithCreds, organizationId, inboxId, rules)

	// Enable deduplication on the scenario (scenario-scoped, so no new draft/publish cycle
	// is needed).
	dedup := true
	scenarioUsecase := usecasesWithCreds.NewScenarioUsecase()
	_, err := scenarioUsecase.UpdateScenario(ctx, models.UpdateScenarioInput{
		Id:                      scenarioId,
		DeduplicateBatchObjects: &dedup,
	})
	if err != nil {
		assert.FailNow(t, "Failed to enable batch deduplication on the scenario", err)
	}

	ingestAccountsBatch(ctx, t, usecasesWithCreds, organizationId, string(userCreds.ActorIdentity.UserId))

	// First run: the object has never been scored, one decision is created.
	se1 := runScheduledExecutionToCompletion(ctx, t, usecasesWithCreds, organizationId, scenarioId, scenarioIterationId, nil)
	assert.Equal(t, 1, se1.NumberOfCreatedDecisions, "first run should create 1 decision")
	assert.Equal(t, 1, se1.NumberOfEvaluatedDecisions, "first run should evaluate 1 object")
	assert.EqualValues(t, 1, se1.ManifestRowsProcessed, "first run should have consumed the single manifest row")

	// Second run over the same object: it must be skipped before evaluation, not just
	// before decision creation.
	se2 := runScheduledExecutionToCompletion(ctx, t, usecasesWithCreds, organizationId, scenarioId, scenarioIterationId, nil)
	assert.Equal(t, 0, se2.NumberOfCreatedDecisions, "second run must not recreate a decision for an already-scored object")
	assert.Equal(t, 0, se2.NumberOfEvaluatedDecisions, "second run must skip the already-scored object before evaluation")
	// ManifestRowsProcessed still advances even though the object was skipped before
	// evaluation: it counts consumed manifest rows, not evaluations, which is the whole
	// point of exposing it (see dto.ScheduledExecutionDto.ManifestRowsProcessed) -- it is
	// the progress signal that keeps moving when NumberOfEvaluatedDecisions is frozen at 0.
	assert.EqualValues(t, 1, se2.ManifestRowsProcessed, "second run should still report the manifest row as processed despite skipping it")

	decisionsUsecase := usecasesWithCreds.NewDecisionUsecase()
	decisions, err := decisionsUsecase.ListDecisions(ctx, organizationId,
		models.NewDefaultPaginationAndSorting("created_at"),
		dto.DecisionFilters{ScenarioIds: []string{scenarioId}},
	)
	if err != nil {
		assert.FailNow(t, "Error while listing decisions", err)
	}
	assert.Equalf(t, 1, len(decisions.Decisions),
		"expected exactly 1 decision across both runs, got %d", len(decisions.Decisions))

	// Third run, with the scenario's dedup default overridden to false for this run only:
	// the same object must be scored again, proving the override actually takes effect and
	// is not just cosmetic on top of the scenario default.
	noDedup := false
	se3 := runScheduledExecutionToCompletion(ctx, t, usecasesWithCreds, organizationId, scenarioId, scenarioIterationId, &noDedup)
	assert.Equal(t, 1, se3.NumberOfCreatedDecisions, "overridden run must recreate a decision despite the object being already scored")
	assert.Equal(t, 1, se3.NumberOfEvaluatedDecisions, "overridden run must evaluate the object instead of skipping it")
	assert.False(t, se3.DeduplicateObjects, "the execution must record the overridden value, not the scenario's default")

	decisions, err = decisionsUsecase.ListDecisions(ctx, organizationId,
		models.NewDefaultPaginationAndSorting("created_at"),
		dto.DecisionFilters{ScenarioIds: []string{scenarioId}},
	)
	if err != nil {
		assert.FailNow(t, "Error while listing decisions", err)
	}
	assert.Equalf(t, 2, len(decisions.Decisions),
		"expected 2 decisions after the overridden third run, got %d", len(decisions.Decisions))
}

// TestBatchExecutionDeduplicationRetroactive proves that scenario_scored_objects is
// populated for every v2 batch execution regardless of whether that run enforces
// deduplication: an object scored while the setting is off must already be excluded the
// first time the setting is turned on, with no separate backfill pass required.
func TestBatchExecutionDeduplicationRetroactive(t *testing.T) {
	t.Setenv("ENABLE_BATCH_EXECUTION_V2", "all")

	ctx := context.Background()
	ctx = utils.StoreLoggerInContext(ctx, utils.NewLogger("text"))
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

	userCreds, _, inboxId := setupOrgAndCreds(ctx, t, "test org with batch dedup retroactive")
	organizationId := userCreds.OrganizationId
	usecasesWithCreds := generateUsecaseWithCreds(testUsecases, userCreds)

	rules := getRulesForBatchTest()
	// Deduplication is left at its default (off) here: setupScenarioAndPublish does not
	// touch it.
	scenarioId, scenarioIterationId := setupScenarioAndPublish(ctx, t, usecasesWithCreds, organizationId, inboxId, rules)

	ingestAccountsBatch(ctx, t, usecasesWithCreds, organizationId, string(userCreds.ActorIdentity.UserId))

	// First run: deduplication is off, so this is ordinary behaviour -- but the object
	// must still be recorded into scenario_scored_objects in the background.
	se1 := runScheduledExecutionToCompletion(ctx, t, usecasesWithCreds, organizationId, scenarioId, scenarioIterationId, nil)
	assert.Equal(t, 1, se1.NumberOfCreatedDecisions, "first run should create 1 decision")
	assert.False(t, se1.DeduplicateObjects, "first run should not enforce dedup, only record")

	// Enable deduplication now, after the object was already processed once with it off.
	dedup := true
	scenarioUsecase := usecasesWithCreds.NewScenarioUsecase()
	_, err := scenarioUsecase.UpdateScenario(ctx, models.UpdateScenarioInput{
		Id:                      scenarioId,
		DeduplicateBatchObjects: &dedup,
	})
	if err != nil {
		assert.FailNow(t, "Failed to enable batch deduplication on the scenario", err)
	}

	// Second run: the first time dedup is enforced for this scenario. If recording during
	// the first (non-enforcing) run had not happened, this would re-score the object and
	// create a second decision -- exactly the gap this behaviour closes.
	se2 := runScheduledExecutionToCompletion(ctx, t, usecasesWithCreds, organizationId, scenarioId, scenarioIterationId, nil)
	assert.Equal(t, 0, se2.NumberOfCreatedDecisions, "object recorded during the earlier non-enforcing run must already be excluded")
	assert.Equal(t, 0, se2.NumberOfEvaluatedDecisions, "object must be skipped before evaluation, exactly as if it had been scored under enforcement")
	assert.True(t, se2.DeduplicateObjects, "second run should be the one enforcing dedup")

	decisionsUsecase := usecasesWithCreds.NewDecisionUsecase()
	decisions, err := decisionsUsecase.ListDecisions(ctx, organizationId,
		models.NewDefaultPaginationAndSorting("created_at"),
		dto.DecisionFilters{ScenarioIds: []string{scenarioId}},
	)
	if err != nil {
		assert.FailNow(t, "Error while listing decisions", err)
	}
	assert.Equalf(t, 1, len(decisions.Decisions),
		"expected exactly 1 decision: the object must not be re-scored once dedup is turned on, got %d", len(decisions.Decisions))
}

// runScheduledExecutionToCompletion creates a manual scheduled execution for the given
// scenario/iteration, runs it, and polls until it reaches a terminal status.
// deduplicateOverride is passed through as the per-run dedup override (nil = use the
// scenario's default).
func runScheduledExecutionToCompletion(
	ctx context.Context,
	t *testing.T,
	usecasesWithCreds usecases.UsecasesWithCreds,
	organizationId uuid.UUID,
	scenarioId, scenarioIterationId string,
	deduplicateOverride *bool,
) models.ScheduledExecution {
	scheduledExecUsecase := usecasesWithCreds.NewScheduledExecutionUsecase()

	err := scheduledExecUsecase.CreateScheduledExecution(ctx, models.CreateScheduledExecutionInput{
		OrganizationId:      organizationId,
		ScenarioId:          scenarioId,
		ScenarioIterationId: scenarioIterationId,
		Manual:              true,
		DeduplicateObjects:  deduplicateOverride,
	})
	if err != nil {
		assert.FailNow(t, "Failed to create scheduled execution", err)
	}

	ses, err := scheduledExecUsecase.ListScheduledExecutions(ctx, organizationId, models.ListScheduledExecutionsFilters{
		ScenarioId: scenarioId,
	}, nil)
	if err != nil {
		assert.FailNow(t, "Failed to list scheduled executions", err)
	}
	if len(ses.Executions) == 0 {
		assert.FailNow(t, "Expected at least one scheduled execution", nil)
	}
	// Executions are not guaranteed ordered; take the most recently created pending one.
	se := ses.Executions[0]
	for _, candidate := range ses.Executions {
		if candidate.StartedAt.After(se.StartedAt) {
			se = candidate
		}
	}

	runScheduledExecUsecase := usecasesWithCreds.NewRunScheduledExecution()
	if err := runScheduledExecUsecase.ExecuteScheduledExecutionById(ctx, se.Id); err != nil {
		assert.FailNow(t, "Failed to run scheduled execution", err)
	}

	// v2 batch execution hands off to a coordinator job running on the worker pool, so
	// completion is asynchronous even for a single-object run.
	start := time.Now()
	for time.Since(start) < 10*time.Second {
		se, err = scheduledExecUsecase.GetScheduledExecution(ctx, se.Id)
		if err != nil {
			assert.FailNow(t, "Failed to get scheduled execution", err)
		}
		if se.Status == models.ScheduledExecutionSuccess || se.Status == models.ScheduledExecutionFailure {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if se.Status != models.ScheduledExecutionSuccess {
		assert.FailNow(t, "Scheduled execution did not succeed within allocated 10sec", "Status is %s", se.Status)
	}
	return se
}
