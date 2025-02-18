package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func IfTaskIsCreated(ctx context.Context, client *river.Client[pgx.Tx], timeout time.Duration, tasks ...string) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return false

		default:
			jobs, err := client.JobList(ctx, river.NewJobListParams().Kinds(tasks...))

			if err == nil && len(jobs.Jobs) >= len(tasks) {
				return true
			}

			time.Sleep(50 * time.Millisecond)
		}
	}
}

func WaitUntilAllTasksDone(t *testing.T, ctx context.Context, client *river.Client[pgx.Tx], timeout time.Duration, tasks ...string) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Errorf("reached %s timeout while waiting for async tasks %s to complete", timeout, tasks)
			return

		default:
			jobs, err := client.JobList(ctx, river.NewJobListParams().Kinds(tasks...).States(rivertype.JobStateCompleted))

			if err == nil && len(jobs.Jobs) >= len(tasks) {
				return
			}

			time.Sleep(50 * time.Millisecond)
		}
	}
}
