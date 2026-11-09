//go:build sar_live

package repositories

// Runs the SAR repository against a migrated PostgreSQL:
//   SAR_LIVE_DSN=postgres://postgres:marble@127.0.0.1:55432/marble go test -tags sar_live ./repositories -run TestSarRepositoryLive

import (
	"context"
	"os"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestSarRepositoryLive(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("SAR_LIVE_DSN"))
	require.NoError(t, err)
	defer pool.Close()
	exec, err := NewExecutorGetter(pool, nil, nil, noop.NewTracerProvider()).GetExecutor(ctx, models.DATABASE_SCHEMA_TYPE_MARBLE, nil)
	require.NoError(t, err)
	org, other, caseID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, o := range []string{org, other} {
		_, err = pool.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2)`, o, "t-"+o[:8])
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, tenant_id) VALUES ($1, $2, $1)`, o, "org-"+o[:8])
		require.NoError(t, err)
	}
	inbox := uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO inboxes (id, name, organization_id) VALUES ($1, 'i', $2)`, inbox, org)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO cases (id, org_id, name, inbox_id) VALUES ($1, $2, 'c', $3)`, caseID, org, inbox)
	require.NoError(t, err)
	repo := NewMarbleDbRepository(false, 0)

	require.NoError(t, repo.SaveSarConfiguration(ctx, exec, models.SarConfiguration{OrgID: org, Rates: map[string]string{"USD/EUR": "0.92"}}, 0))
	cfg, err := repo.GetSarConfiguration(ctx, exec, org)
	require.NoError(t, err)
	require.Equal(t, int64(1), cfg.Revision)
	require.ErrorIs(t, repo.SaveSarConfiguration(ctx, exec, cfg, 0), models.ConflictError)

	p := models.SarPivot{CaseID: caseID, OrgID: org, Suspicion: models.SarSuspicion{Analysis: &models.SarText{V: "x", P: &models.SarProvenance{Kind: "manual"}}}}
	require.NoError(t, repo.SaveSarPivot(ctx, exec, p, 0))
	require.ErrorIs(t, repo.SaveSarPivot(ctx, exec, models.SarPivot{CaseID: caseID, OrgID: other}, 0), models.ConflictError, "case of another org")
	got, found, err := repo.GetSarPivot(ctx, exec, org, caseID, false)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "x", got.Suspicion.Analysis.V)
	require.NoError(t, repo.SaveSarPivot(ctx, exec, got, 1))
	require.ErrorIs(t, repo.SaveSarPivot(ctx, exec, got, 1), models.ConflictError)
	_, found, _ = repo.GetSarPivot(ctx, exec, other, caseID, false)
	require.False(t, found, "isolation")

	rep := models.SarReport{ID: uuid.NewString(), CaseID: caseID, OrgID: org, FormatKey: "goaml:LU:5.0.2", Status: "ready"}
	require.NoError(t, repo.SaveSarReport(ctx, exec, rep, 0))
	require.NoError(t, repo.SaveSarReport(ctx, exec, rep, 1))
	reps, err := repo.ListSarReports(ctx, exec, org, caseID)
	require.NoError(t, err)
	require.Len(t, reps, 1)
	require.Equal(t, int64(2), reps[0].Revision)
	_, err = repo.GetSarReport(ctx, exec, other, caseID, rep.ID)
	require.Error(t, err)

	require.NoError(t, repo.AddSarRevision(ctx, exec, models.SarRevision{OrgID: org, CaseID: caseID, Object: "pivot", Revision: 2, Origin: "manual", ActorID: "u", Summary: "s", Patch: []byte(`[{"op":"set"}]`)}, p))
	revs, err := repo.ListSarRevisions(ctx, exec, org, caseID, "pivot")
	require.NoError(t, err)
	require.Len(t, revs, 1)

	run := models.SarRun{ID: uuid.NewString(), OrgID: org, CaseID: caseID, Status: "queued"}
	require.NoError(t, repo.CreateSarRun(ctx, exec, run))
	require.ErrorIs(t, repo.CreateSarRun(ctx, exec, models.SarRun{ID: uuid.NewString(), OrgID: org, CaseID: caseID, Status: "queued"}), models.ConflictError, "one active run")
	run.Status = "completed"
	require.NoError(t, repo.SaveSarRun(ctx, exec, run))
	gotRun, err := repo.GetSarRun(ctx, exec, org, caseID, run.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", gotRun.Status)

	e := models.SarMemoryEntry{OrgID: org, FormatKey: "goaml:CH:2025.3", Code: "MROS-A05", Path: "/report/report_indicators", Cause: "deux M"}
	require.NoError(t, repo.UpsertSarMemory(ctx, exec, e))
	e.Fix, e.Corrected = "indicators = 0002M", 1
	require.NoError(t, repo.UpsertSarMemory(ctx, exec, e))
	mem, err := repo.ListSarMemory(ctx, exec, org, "goaml:CH:2025.3", 10)
	require.NoError(t, err)
	require.Len(t, mem, 1)
	require.Equal(t, 2, mem[0].Frequency)
	require.Equal(t, 1, mem[0].Corrected)
	require.Equal(t, "indicators = 0002M", mem[0].Fix)

	m := models.SarChatMessage{ID: uuid.NewString(), OrgID: org, CaseID: caseID, ReportID: rep.ID, Role: "user", Content: "hello"}
	require.NoError(t, repo.AddSarChatMessage(ctx, exec, m))
	m.Status = "applied"
	require.NoError(t, repo.UpdateSarChatMessage(ctx, exec, m))
	msgs, err := repo.ListSarChatMessages(ctx, exec, org, rep.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Equal(t, "applied", msgs[0].Status)
}
