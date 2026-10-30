package worker_jobs

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type fakeScheduledExecutionFinisher struct {
	updated   bool
	updateErr error
	acked     []string
}

func (f *fakeScheduledExecutionFinisher) UpdateScheduledExecutionStatus(
	ctx context.Context,
	exec repositories.Executor,
	input models.UpdateScheduledExecutionStatusInput,
) (bool, error) {
	return f.updated, f.updateErr
}

func (f *fakeScheduledExecutionFinisher) AckScheduledExecutionCancel(ctx context.Context, exec repositories.Executor, id string) error {
	f.acked = append(f.acked, id)
	return nil
}

func TestFinishScheduledExecution(t *testing.T) {
	ctx := context.Background()

	t.Run("final status applied: no acknowledgement", func(t *testing.T) {
		repo := &fakeScheduledExecutionFinisher{updated: true}
		err := finishScheduledExecution(ctx, repo, nil, "id", models.ScheduledExecutionSuccess)
		assert.NoError(t, err)
		assert.Empty(t, repo.acked)
	})

	t.Run("final status not applied: cancellation acknowledged", func(t *testing.T) {
		repo := &fakeScheduledExecutionFinisher{updated: false}
		err := finishScheduledExecution(ctx, repo, nil, "id", models.ScheduledExecutionFailure)
		assert.NoError(t, err)
		assert.Equal(t, []string{"id"}, repo.acked)
	})

	t.Run("update error: returned, no acknowledgement", func(t *testing.T) {
		updateErr := errors.New("db down")
		repo := &fakeScheduledExecutionFinisher{updateErr: updateErr}
		err := finishScheduledExecution(ctx, repo, nil, "id", models.ScheduledExecutionSuccess)
		assert.ErrorIs(t, err, updateErr)
		assert.Empty(t, repo.acked)
	})
}
