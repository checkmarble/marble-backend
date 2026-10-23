package repositories

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

type recordingExecutor struct {
	queries []string
}

func (e *recordingExecutor) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	e.queries = append(e.queries, sql)
	return pgconn.NewCommandTag(""), nil
}

func TestInjectDbSessionConfig(t *testing.T) {
	t.Run("sets audit configuration on writes and transactions", func(t *testing.T) {
		for _, query := range []string{"UPDATE decisions SET review_status = 'ok'", ""} {
			exec := &recordingExecutor{}

			_, err := injectDbSessionConfig(context.Background(), exec, query)

			assert.NoError(t, err)
			assert.Len(t, exec.queries, 1)
			assert.True(t, strings.HasPrefix(exec.queries[0], "SELECT set_config("))
		}
	})

	t.Run("skips audit configuration when disabled in context", func(t *testing.T) {
		ctx := WithoutAudit(context.Background())

		for _, query := range []string{"UPDATE decisions SET review_status = 'ok'", ""} {
			exec := &recordingExecutor{}

			_, err := injectDbSessionConfig(ctx, exec, query)

			assert.NoError(t, err)
			assert.Empty(t, exec.queries)
		}
	})
}
