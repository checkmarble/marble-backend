package cmd

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSingleJobCredentials(t *testing.T) {
	parent := context.Background()
	org := uuid.New()
	ctx, err := singleJobContext(parent, "case_review", `{"org_id":"`+org.String()+`","case_id":"`+uuid.NewString()+`"}`)
	require.NoError(t, err)
	creds, ok := utils.CredentialsFromCtx(ctx)
	require.True(t, ok)
	require.Equal(t, org, creds.OrganizationId)
	require.Equal(t, []models.Role{models.SYSTEM}, creds.Roles)
	job := singleJobCreate[models.CaseReviewArgs](ctx, `{"case_id":"`+uuid.NewString()+`"}`)
	require.Equal(t, org.String(), job.Queue)
	_, ok = utils.CredentialsFromCtx(parent)
	require.False(t, ok)

	for _, args := range []string{"", `{}`, `{"org_id":"invalid"}`, `{"org_id":"00000000-0000-0000-0000-000000000000"}`} {
		// A missing scope must be rejected before constructing or running a worker.
		require.Error(t, singleJobRun(parent, usecases.UsecasesWithCreds{}, "", WorkerConfig{}, infra.GcpConfig{}, "case_review", args))
	}
	for _, tc := range []struct{ name, queue string }{
		{"analytics_merge", "analytics_merge"},
		{"send_billing_event", models.BILLING_QUEUE_NAME},
		{"continuous_screening_scan_dataset_updates", models.CONTINUOUS_SCREENING_DATASET_UPDATE_QUEUE_NAME},
		{"webhook_cleanup", worker_jobs.WEBHOOK_CLEANUP_QUEUE},
		{"async_decision_execution_cleanup", worker_jobs.ASYNC_DECISION_CLEANUP_QUEUE},
	} {
		ctx, err := singleJobContext(ctx, tc.name, `{}`)
		require.NoError(t, err)
		creds, _ := utils.CredentialsFromCtx(ctx)
		require.Equal(t, uuid.Nil, creds.OrganizationId)
		require.Equal(t, tc.queue, singleJobQueue(ctx, tc.name))
	}
}
