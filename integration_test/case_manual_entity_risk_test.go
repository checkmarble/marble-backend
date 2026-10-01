package integration

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/require"
)

func TestManualCaseEntitiesConfirmedRisks(t *testing.T) {
	ctx := context.Background()
	repo := testUsecases.Repositories.MarbleDbRepository
	err := testUsecases.Repositories.ExecutorGetter.Transaction(ctx, models.DATABASE_SCHEMA_TYPE_MARBLE, nil, func(tx repositories.Transaction) error {
		// Temporary tables exercise the actual repository queries without modifying
		// the shared integration fixtures or needing customer ingestion.
		_, err := tx.Exec(ctx, `
			create temporary table cases (like marble.cases including defaults) on commit drop;
			create temporary table case_manual_entities (like marble.case_manual_entities including defaults) on commit drop;
			create temporary table data_model_tables (id uuid, organization_id uuid, name text) on commit drop;
			create temporary table data_model_links (id uuid, parent_table_id uuid) on commit drop;
			create temporary table data_model_pivots (
				id uuid, organization_id uuid, base_table_id uuid, field_id uuid,
				path_link_ids uuid[], deleted_at timestamp
			) on commit drop;
			create temporary table decisions (case_id uuid, org_id uuid, pivot_id uuid, pivot_value text) on commit drop;
		`)
		require.NoError(t, err)
		org, otherOrg := pure_utils.NewId(), pure_utils.NewId()
		caseID, tableID, pivotID := pure_utils.NewId(), pure_utils.NewId(), pure_utils.NewId()
		exec := func(sql string, args ...any) {
			_, err := tx.Exec(ctx, sql, args...)
			require.NoError(t, err)
		}
		check := func(want bool) {
			got, err := repo.ObjectHasConfirmedRisks(ctx, tx, org, "customers", "c-123")
			require.NoError(t, err)
			require.Equal(t, want, got)
		}
		exec(`insert into cases (id, org_id, inbox_id, name, status, outcome)
			values ($1, $2, $3, 'manual investigation', 'pending', 'unset')`, caseID, org, pure_utils.NewId())
		exec(`insert into case_manual_entities (id, org_id, case_id, table_name, object_id)
			values ($1, $2, $3, 'customers', 'c-123')`, pure_utils.NewId(), org, caseID)
		check(false)
		manualCases, err := repo.GetCasesRelatedToObject(ctx, tx, org, "customers", "c-123")
		require.NoError(t, err)
		require.Len(t, manualCases, 1)
		require.Equal(t, caseID.String(), manualCases[0].Id)
		exec(`update cases set status = 'investigating', outcome = 'confirmed_risk' where id = $1`, caseID)
		check(true) // confirmed_risk applies even while the case is open.
		exec(`update cases set status = 'closed' where id = $1`, caseID)
		check(true)
		exec(`update cases set status = 'pending' where id = $1`, caseID)
		check(true)
		exec(`update cases set outcome = 'false_positive' where id = $1`, caseID)
		check(false)
		exec(`update cases set outcome = 'confirmed_risk' where id = $1`, caseID)
		exec(`delete from case_manual_entities where case_id = $1`, caseID)
		check(false)

		// Neither another table's matching ID nor another organization's links count.
		exec(`insert into case_manual_entities (id, org_id, case_id, table_name, object_id)
			values ($1, $2, $3, 'partners', 'c-123'), ($4, $5, $3, 'customers', 'c-123')`,
			pure_utils.NewId(), org, caseID, pure_utils.NewId(), otherOrg)
		check(false)
		foreignCase := pure_utils.NewId()
		exec(`insert into cases (id, org_id, inbox_id, name, status, outcome)
			values ($1, $2, $3, 'foreign investigation', 'closed', 'confirmed_risk')`, foreignCase, otherOrg, pure_utils.NewId())
		exec(`insert into case_manual_entities (id, org_id, case_id, table_name, object_id)
			values ($1, $2, $3, 'customers', 'c-123')`, pure_utils.NewId(), otherOrg, foreignCase)
		check(false)

		// Decision pivots (including deleted definitions) remain supported; a case
		// linked through both sources is listed once.
		exec(`insert into data_model_tables values ($1, $2, 'customers')`, tableID, org)
		exec(`insert into data_model_pivots (id, organization_id, base_table_id, field_id, deleted_at)
			values ($1, $2, $3, $4, now())`, pivotID, org, tableID, pure_utils.NewId())
		exec(`insert into decisions values ($1, $2, $3, 'c-123')`, caseID, org, pivotID)
		check(true)
		exec(`insert into case_manual_entities (id, org_id, case_id, table_name, object_id)
			values ($1, $2, $3, 'customers', 'c-123')`, pure_utils.NewId(), org, caseID)
		cases, err := repo.GetCasesRelatedToObject(ctx, tx, org, "customers", "c-123")
		require.NoError(t, err)
		require.Len(t, cases, 1)
		require.Equal(t, caseID.String(), cases[0].Id)
		exec(`delete from case_manual_entities where org_id = $1 and table_name = 'customers'`, org)
		check(true)
		return nil
	})
	require.NoError(t, err)
}
