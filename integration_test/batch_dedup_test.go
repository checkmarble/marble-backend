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
	se1 := runScheduledExecutionToCompletion(ctx, t, usecasesWithCreds, organizationId, scenarioId, scenarioIterationId)
	assert.Equal(t, 1, se1.NumberOfCreatedDecisions, "first run should create 1 decision")
	assert.Equal(t, 1, se1.NumberOfEvaluatedDecisions, "first run should evaluate 1 object")

	// Second run over the same object: it must be skipped before evaluation, not just
	// before decision creation.
	se2 := runScheduledExecutionToCompletion(ctx, t, usecasesWithCreds, organizationId, scenarioId, scenarioIterationId)
	assert.Equal(t, 0, se2.NumberOfCreatedDecisions, "second run must not recreate a decision for an already-scored object")
	assert.Equal(t, 0, se2.NumberOfEvaluatedDecisions, "second run must skip the already-scored object before evaluation")

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
}

// runScheduledExecutionToCompletion creates a manual scheduled execution for the given
// scenario/iteration, runs it, and polls until it reaches a terminal status.
func runScheduledExecutionToCompletion(
	ctx context.Context,
	t *testing.T,
	usecasesWithCreds usecases.UsecasesWithCreds,
	organizationId uuid.UUID,
	scenarioId, scenarioIterationId string,
) models.ScheduledExecution {
	scheduledExecUsecase := usecasesWithCreds.NewScheduledExecutionUsecase()

	err := scheduledExecUsecase.CreateScheduledExecution(ctx, models.CreateScheduledExecutionInput{
		OrganizationId:      organizationId,
		ScenarioId:          scenarioId,
		ScenarioIterationId: scenarioIterationId,
		Manual:              true,
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
