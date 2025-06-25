package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/jackc/pgx/v5"
)

func (repo *MarbleDbRepository) SelectCasesWithPivot(
	ctx context.Context,
	exec Executor,
	filters models.DecisionWorkflowFilters,
) ([]models.CaseMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	// Both sides of the query should be ordered by case_id so that a merge join is possible
	// NB: may select a snoozed case, in which case adding the decision will automatically unsnooze it.
	sql := NewQueryBuilder().
		Select(columnsNames("c", []string{"id", "status", "created_at", "org_id"})...).
		From(dbmodels.TABLE_CASES+" c").
		InnerJoin("(select distinct case_id from decisions where org_id = ? and pivot_value = ? and case_id is not null order by case_id) d on c.id = d.case_id", filters.OrganizationId, filters.PivotValue).
		Where(squirrel.Eq{
			"c.org_id": filters.OrganizationId,
			"c.status": []string{"pending", "investigating"},
		})

	if filters.InboxId != nil {
		sql = sql.Where("c.inbox_id = ?", filters.InboxId)
	}

	query, args, _ := sql.ToSql()

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	cases, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.CaseMetadata, error) {
		var c models.CaseMetadata
		err := row.Scan(&c.Id, &c.Status, &c.CreatedAt, &c.OrganizationId)
		return c, err
	})

	return cases, err
}

func (repo *MarbleDbRepository) CountDecisionsByCaseIds(
	ctx context.Context,
	exec Executor,
	organizationId string,
	caseIds []string,
) (map[string]int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := `SELECT case_id, COUNT(DISTINCT pivot_value) AS nb FROM decisions WHERE org_id = $1 AND case_id = ANY($2) GROUP BY case_id`
	rows, err := exec.Query(ctx, query, organizationId, caseIds)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	var caseId string
	var count int
	_, err = pgx.ForEachRow(rows, []any{&caseId, &count}, func() error {
		counts[caseId] = count
		return nil
	})

	return counts, err
}
