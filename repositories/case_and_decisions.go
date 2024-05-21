package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
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

	query := `SELECT c.id, c.status, c.created_at, c.org_id
	FROM cases AS c
	INNER JOIN decisions AS d ON c.id = d.case_id
	WHERE c.org_id = $1 
		AND c.status IN ($2)
		AND c.inboxId = $3
		AND d.pivot_value = $4
	`

	rows, err := exec.Query(ctx, query, filters.OrganizationId, filters.Statuses, filters.InboxId, filters.PivotValue)
	if err != nil {
		return nil, err
	}
	cases, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (
		models.CaseMetadata, error,
	) {
		var c models.CaseMetadata
		err := row.Scan(&c.Id, &c.Status, &c.CreatedAt, &c.OrganizationId)
		return c, err
	})

	return cases, err
}

func (repo *MarbleDbRepository) CountDecisionsByCaseIds(ctx context.Context, exec Executor, caseIds []string) (map[string]int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := `SELECT case_id, COUNT(*) AS nb FROM decisions WHERE case_id IN ($1) GROUP BY case_id`
	rows, err := exec.Query(ctx, query, caseIds)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	var caseId string
	var count int
	_, err = pgx.ForEachRow(rows, []any{caseId, count}, func() error {
		counts[caseId] = count
		return nil
	})

	return counts, err
}