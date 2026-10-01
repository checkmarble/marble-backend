package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/stretchr/testify/require"
)

// Exercise the repository SQL against Postgres. Temporary tables isolate these
// fixtures from the background workers and keep the real case/link column types.
func TestCaseEntityConfirmedRisk(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	admin := generateUsecaseWithCredForMarbleAdmin(testUsecases)
	repo := admin.Repositories.MarbleDbRepository
	orgId, otherOrgId := uuid.New(), uuid.New()
	caseId := uuid.NewString()
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "customer-1"}

	err := admin.NewTransactionFactory().Transaction(ctx, func(tx repositories.Transaction) error {
		for _, sql := range []string{
			`create temporary table cases (like marble.cases including all) on commit drop`,
			`create temporary table case_manual_entities (like marble.case_manual_entities including all) on commit drop`,
			`create temporary table data_model_tables (id uuid, organization_id uuid, name text) on commit drop`,
			`create temporary table data_model_links (id uuid, parent_table_id uuid) on commit drop`,
			`create temporary table data_model_pivots (id uuid, organization_id uuid, base_table_id uuid, field_id uuid, path_link_ids uuid[], deleted_at timestamptz) on commit drop`,
			`create temporary table decisions (org_id uuid, case_id uuid, pivot_id uuid, pivot_value text) on commit drop`,
		} {
			_, err := tx.Exec(ctx, sql)
			require.NoError(t, err)
		}
		require.NoError(t, repo.CreateCase(ctx, tx, models.CreateCaseAttributes{
			OrganizationId: orgId, InboxId: uuid.New(), Name: "Manual case", Type: models.CaseTypeDecision,
		}, caseId))
		link, err := repo.InsertCaseManualEntity(ctx, tx, orgId, caseId, ref)
		require.NoError(t, err)
		require.NotNil(t, link)

		check := func(wantRisk bool, wantCases int) {
			t.Helper()
			hasRisk, err := repo.ObjectHasConfirmedRisks(ctx, tx, orgId, ref.TableName, ref.ObjectId)
			require.NoError(t, err)
			require.Equal(t, wantRisk, hasRisk)
			cases, err := repo.GetCasesRelatedToObject(ctx, tx, orgId, ref.TableName, ref.ObjectId)
			require.NoError(t, err)
			require.Len(t, cases, wantCases)
		}

		// A manual-only case needs neither a decision nor a configured pivot.
		check(false, 1)
		for _, status := range []models.CaseStatus{models.CasePending, models.CaseInvestigating, models.CaseClosed, models.CaseInvestigating} {
			require.NoError(t, repo.UpdateCase(ctx, tx, models.UpdateCaseAttributes{
				Id: caseId, Status: status, Outcome: models.CaseConfirmedRisk,
			}))
			check(true, 1)
		}
		for _, outcome := range []models.CaseOutcome{models.CaseFalsePositive, models.CaseValuableAlert, models.CaseOutcomeUnset} {
			require.NoError(t, repo.UpdateCase(ctx, tx, models.UpdateCaseAttributes{Id: caseId, Outcome: outcome}))
			check(false, 1)
		}
		require.NoError(t, repo.UpdateCase(ctx, tx, models.UpdateCaseAttributes{Id: caseId, Outcome: models.CaseConfirmedRisk}))

		// The same ID in another table or organization must not inherit this risk.
		for _, target := range []struct {
			org   uuid.UUID
			table string
			id    string
		}{
			{otherOrgId, ref.TableName, ref.ObjectId},
			{orgId, "companies", ref.ObjectId},
			{orgId, ref.TableName, "customer-2"},
		} {
			hasRisk, err := repo.ObjectHasConfirmedRisks(ctx, tx, target.org, target.table, target.id)
			require.NoError(t, err)
			require.False(t, hasRisk)
			cases, err := repo.GetCasesRelatedToObject(ctx, tx, target.org, target.table, target.id)
			require.NoError(t, err)
			require.Empty(t, cases)
		}
		_, err = repo.DeleteCaseManualEntity(ctx, tx, orgId, caseId, ref)
		require.NoError(t, err)
		check(false, 0)

		// Scope cases as well as links: even an inconsistent cross-org link
		// must not expose a foreign case or its confirmed outcome.
		foreignCaseId := uuid.NewString()
		require.NoError(t, repo.CreateCase(ctx, tx, models.CreateCaseAttributes{
			OrganizationId: otherOrgId, InboxId: uuid.New(), Name: "Foreign case", Type: models.CaseTypeDecision,
		}, foreignCaseId))
		require.NoError(t, repo.UpdateCase(ctx, tx, models.UpdateCaseAttributes{Id: foreignCaseId, Outcome: models.CaseConfirmedRisk}))
		_, err = repo.InsertCaseManualEntity(ctx, tx, orgId, foreignCaseId, ref)
		require.NoError(t, err)
		check(false, 0)

		// Historical decisions still count through both direct and linked pivots,
		// even after those pivots are soft-deleted.
		tableId, linkId := uuid.New(), uuid.New()
		_, err = tx.Exec(ctx, `insert into data_model_tables values ($1, $2, $3)`, tableId, orgId, ref.TableName)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `insert into data_model_links values ($1, $2)`, linkId, tableId)
		require.NoError(t, err)
		for _, direct := range []bool{true, false} {
			_, err = tx.Exec(ctx, `truncate decisions, data_model_pivots`)
			require.NoError(t, err)
			pivotId := uuid.New()
			var fieldId *uuid.UUID
			var path []uuid.UUID
			if direct {
				id := uuid.New()
				fieldId = &id
			} else {
				path = []uuid.UUID{linkId}
			}
			_, err = tx.Exec(ctx, `insert into data_model_pivots values ($1, $2, $3, $4, $5, now())`, pivotId, orgId, tableId, fieldId, path)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `insert into decisions values ($1, $2, $3, $4), ($1, $2, $3, $4)`, orgId, caseId, pivotId, ref.ObjectId)
			require.NoError(t, err)
			check(true, 1)
			_, err = repo.InsertCaseManualEntity(ctx, tx, orgId, caseId, ref)
			require.NoError(t, err)
			check(true, 1) // Manual and decision links must not duplicate the case.
			_, err = repo.DeleteCaseManualEntity(ctx, tx, orgId, caseId, ref)
			require.NoError(t, err)
			check(true, 1) // Removing the manual link preserves the decision link.
		}

		// The risk predicate is not limited to the 200 most recent cases shown
		// by the Customer Hub. Keep the confirmed case older than that window.
		_, err = tx.Exec(ctx, `update cases set created_at = now() - interval '1 year' where id = $1`, caseId)
		require.NoError(t, err)
		for range 200 {
			id := uuid.NewString()
			require.NoError(t, repo.CreateCase(ctx, tx, models.CreateCaseAttributes{
				OrganizationId: orgId, InboxId: uuid.New(), Name: "Recent case", Type: models.CaseTypeDecision,
			}, id))
			_, err = repo.InsertCaseManualEntity(ctx, tx, orgId, id, ref)
			require.NoError(t, err)
		}
		check(true, 200)
		cases, err := repo.GetCasesRelatedToObject(ctx, tx, orgId, ref.TableName, ref.ObjectId)
		require.NoError(t, err)
		for _, c := range cases {
			require.NotEqual(t, caseId, c.Id)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestCaseScoreComputationsAreNotThrottled(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	// A producer-only client and a unique organization queue keep the jobs from
	// running while we verify scheduling and transaction rollback.
	client, err := river.NewClient(riverpgxv5.New(pgPool), &river.Config{})
	require.NoError(t, err)
	queue := repositories.NewTaskQueueRepository(client)
	admin := generateUsecaseWithCredForMarbleAdmin(testUsecases)
	record := models.ScoringRecordRef{OrgId: uuid.New(), RecordType: "customers", RecordId: "customer-1"}
	t.Cleanup(func() {
		_, err := pgPool.Exec(ctx, `delete from river_job where queue = $1`, record.OrgId.String())
		require.NoError(t, err)
	})
	enqueue := func() {
		t.Helper()
		require.NoError(t, admin.NewTransactionFactory().Transaction(ctx, func(tx repositories.Transaction) error {
			return queue.EnqueueScoreComputationForCase(ctx, tx, record)
		}))
	}
	count := func(want int) {
		t.Helper()
		var got int
		require.NoError(t, pgPool.QueryRow(ctx, `select count(*) from river_job where queue = $1`, record.OrgId.String()).Scan(&got))
		require.Equal(t, want, got)
	}
	enqueue()
	for i, state := range []string{"available", "running", "completed"} {
		_, err := pgPool.Exec(ctx, `update river_job set state = $2::river_job_state,
			finalized_at = case when $2 = 'completed' then now() else null end
			where queue = $1`, record.OrgId.String(), state)
		require.NoError(t, err)
		enqueue()
		count(i + 2)
	}
	rollback := fmt.Errorf("case mutation failed")
	err = admin.NewTransactionFactory().Transaction(ctx, func(tx repositories.Transaction) error {
		require.NoError(t, queue.EnqueueScoreComputationForCase(ctx, tx, record))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	count(4)
}
