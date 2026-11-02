package jobs

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
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

func TestOrganizationCredentialsMiddleware(t *testing.T) {
	orgID := uuid.New()
	parentCreds := models.Credentials{OrganizationId: uuid.New(), Roles: []models.Role{models.VIEWER}}
	parent := context.WithValue(context.Background(), utils.ContextKeyCredentials, parentCreds)
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	wantErr := errors.New("job failed")
	middleware := NewOrganizationCredentialsMiddleware()

	for _, tc := range []struct {
		queue string
		orgID uuid.UUID
	}{
		{orgID.String(), orgID},
		{"analytics_merge", uuid.Nil},
		{models.CONTINUOUS_SCREENING_DATASET_UPDATE_QUEUE_NAME, uuid.Nil},
	} {
		t.Run(tc.queue, func(t *testing.T) {
			called := false
			err := middleware.Work(ctx, &rivertype.JobRow{Queue: tc.queue}, func(jobCtx context.Context) error {
				called = true
				creds, found := utils.CredentialsFromCtx(jobCtx)
				require.True(t, found)
				require.Equal(t, tc.orgID, creds.OrganizationId)
				require.Equal(t, []models.Role{models.SYSTEM}, creds.Roles)
				require.Empty(t, creds.ActorIdentity)
				require.Equal(t, ctx.Done(), jobCtx.Done())
				return wantErr
			})
			require.True(t, called)
			require.ErrorIs(t, err, wantErr)
			got, _ := utils.CredentialsFromCtx(ctx)
			require.Equal(t, parentCreds, got)
		})
	}
}
