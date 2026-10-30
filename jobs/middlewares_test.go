package jobs

import (
	"context"
	"log/slog"
	"testing"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river/rivertype"
)

func TestLoggerMiddlewareStoresExecutionSource(t *testing.T) {
	middleware := NewLoggerMiddleware(slog.Default())
	job := &rivertype.JobRow{ID: 123, Kind: "async_decision"}

	err := middleware.Work(context.Background(), job, func(ctx context.Context) error {
		source, found := utils.ExecutionSourceFromContext(ctx)
		if !found {
			t.Fatal("execution source not found")
		}
		want := utils.ExecutionSource{
			Type:    utils.ExecutionSourceRiverJob,
			JobID:   job.ID,
			JobKind: job.Kind,
		}
		if source != want {
			t.Fatalf("execution source = %+v, want %+v", source, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
