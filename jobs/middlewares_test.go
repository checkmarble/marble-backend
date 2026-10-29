package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
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

func TestCredentialsMiddleware(t *testing.T) {
	parentCreds := models.Credentials{OrganizationId: uuid.New(), Roles: []models.Role{models.ADMIN}}
	parent := context.WithValue(context.Background(), utils.ContextKeyCredentials, parentCreds)
	sentinel := errors.New("worker error")
	for _, queue := range []string{uuid.NewString(), models.METRICS_QUEUE_NAME, models.BILLING_QUEUE_NAME, "analytics_merge", models.CONTINUOUS_SCREENING_DATASET_UPDATE_QUEUE_NAME, worker_jobs.WEBHOOK_CLEANUP_QUEUE, worker_jobs.ASYNC_DECISION_CLEANUP_QUEUE, ""} {
		t.Run(queue, func(t *testing.T) {
			err := (CredentialsMiddleware{}).Work(parent, &rivertype.JobRow{Queue: queue}, func(ctx context.Context) error {
				creds, ok := utils.CredentialsFromCtx(ctx)
				require.True(t, ok)
				want, parseErr := uuid.Parse(queue)
				if parseErr != nil {
					want = uuid.Nil
				}
				require.Equal(t, want, creds.OrganizationId)
				require.Equal(t, []models.Role{models.SYSTEM}, creds.Roles)
				return sentinel
			})
			require.ErrorIs(t, err, sentinel)
			got, _ := utils.CredentialsFromCtx(parent)
			require.Equal(t, parentCreds, got)
		})
	}
}

func TestCredentialsMiddlewareConcurrentCacheIsolation(t *testing.T) {
	server := miniredis.RunT(t)
	client, err := repositories.NewRedisClient(infra.RedisConfig{Address: server.Addr()})
	require.NoError(t, err)
	getter := repositories.NewExecutorGetter(nil, nil, client, nil)
	parent := context.Background()
	repo := repositories.NewMarbleDbRepository(true, 0)
	foreign, err := json.Marshal(models.DataModel{Tables: map[string]models.Table{"foreign": {Name: "foreign"}}})
	require.NoError(t, err)
	server.Set("data-model:true", string(foreign))
	var wg sync.WaitGroup
	for range 20 {
		org := uuid.New()
		cached, err := json.Marshal(models.DataModel{Tables: map[string]models.Table{org.String(): {Name: org.String()}}})
		require.NoError(t, err)
		server.Set(org.String()+":data-model:true", string(cached))
		wg.Go(func() {
			err := (CredentialsMiddleware{}).Work(parent, &rivertype.JobRow{Queue: org.String()}, func(ctx context.Context) error {
				exec, err := getter.GetExecutor(ctx, models.DATABASE_SCHEMA_TYPE_MARBLE, nil)
				if err != nil {
					return err
				}
				for range 10 {
					if got := exec.Cache(ctx).Key("data-model", "true"); got != org.String()+":data-model:true" {
						t.Errorf("cache key = %s, org = %s", got, org)
					}
					nested, cancel := context.WithCancel(ctx)
					creds, ok := utils.CredentialsFromCtx(nested)
					if !ok || creds.OrganizationId != org {
						t.Errorf("nested context lost organization")
					}
					cancel()
				}
				model, err := repo.GetDataModel(ctx, exec, org, true, true)
				if err != nil {
					return err
				}
				if model.Tables[org.String()].Name != org.String() {
					t.Errorf("received another organization's cached model: %+v", model)
				}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	_, ok := utils.CredentialsFromCtx(parent)
	require.False(t, ok)
}
