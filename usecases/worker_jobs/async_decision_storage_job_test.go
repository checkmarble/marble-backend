package worker_jobs

import (
	"context"
	"log/slog"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAsyncDecisionStorer struct {
	stored []models.DecisionBundle
	err    error
}

func (f *fakeAsyncDecisionStorer) StoreDecision(_ context.Context, _ *slog.Logger,
	_ repositories.Transaction, item models.DecisionBundle,
) (models.DecisionWithRuleExecutions, error) {
	f.stored = append(f.stored, item)
	return item.Decision, f.err
}

func makeAsyncDecisionStorageJob(t *testing.T, payload []byte) (*river.Job[models.AsyncDecisionStorageArgs], uuid.UUID) {
	t.Helper()

	decisionId := uuid.New()
	orgId := uuid.New()

	if payload == nil {
		var err error

		payload, err = models.NewTransientDecision(models.DecisionBundle{
			Decision: models.DecisionWithRuleExecutions{
				Decision:       models.Decision{DecisionId: decisionId, OrganizationId: orgId},
				RuleExecutions: []models.RuleExecution{{Rule: models.Rule{Id: "rule1"}, Outcome: "hit"}},
			},
			Scenario: models.Scenario{TriggerObjectType: "transactions"},
			ObjectId: "obj",
		}).Encode()
		require.NoError(t, err)
	}

	return &river.Job[models.AsyncDecisionStorageArgs]{
		JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 5},
		Args:   models.AsyncDecisionStorageArgs{OrgId: orgId, DecisionId: decisionId, Payload: payload},
	}, decisionId
}

func newAsyncDecisionStorageTestWorker(storer *fakeAsyncDecisionStorer) AsyncDecisionStorageWorker {
	return NewAsyncDecisionStorageWorker(
		executor_factory.NewTransactionFactoryStub(executor_factory.NewExecutorFactoryStub()),
		nil,
		storer,
	)
}

func TestAsyncDecisionStorageWorker(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))

	t.Run("stores the decoded decision", func(t *testing.T) {
		storer := &fakeAsyncDecisionStorer{}
		job, decisionId := makeAsyncDecisionStorageJob(t, nil)

		require.NoError(t, newAsyncDecisionStorageTestWorker(storer).Work(ctx, job))
		require.Len(t, storer.stored, 1)

		item := storer.stored[0]

		assert.Equal(t, decisionId, item.Decision.DecisionId)
		assert.Equal(t, "transactions", item.Scenario.TriggerObjectType)
		assert.Equal(t, "obj", item.ObjectId)
		require.Len(t, item.Decision.RuleExecutions, 1)
		assert.Equal(t, "rule1", item.Decision.RuleExecutions[0].Rule.Id)
	})

	t.Run("cancels the job on an invalid payload", func(t *testing.T) {
		storer := &fakeAsyncDecisionStorer{}
		job, _ := makeAsyncDecisionStorageJob(t, []byte("garbage"))

		err := newAsyncDecisionStorageTestWorker(storer).Work(ctx, job)

		var cancelErr *river.JobCancelError
		assert.ErrorAs(t, err, &cancelErr)
		assert.Empty(t, storer.stored)
	})

	t.Run("succeeds if the decision was already stored", func(t *testing.T) {
		storer := &fakeAsyncDecisionStorer{err: &pgconn.PgError{
			Code:           pgerrcode.UniqueViolation,
			ConstraintName: "decisions_pkey",
		}}
		job, _ := makeAsyncDecisionStorageJob(t, nil)

		assert.NoError(t, newAsyncDecisionStorageTestWorker(storer).Work(ctx, job))
	})

	t.Run("returns storage errors for retry", func(t *testing.T) {
		storer := &fakeAsyncDecisionStorer{err: errors.New("boom")}
		job, _ := makeAsyncDecisionStorageJob(t, nil)

		assert.Error(t, newAsyncDecisionStorageTestWorker(storer).Work(ctx, job))
	})
}
