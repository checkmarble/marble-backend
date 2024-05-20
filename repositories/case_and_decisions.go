package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

func (repo *MarbleDbRepository) SelectCasesWithPivot(
	ctx context.Context,
	tx Executor,
	filters models.DecisionWorkflowFilters,
) ([]models.CaseMetadata, error) {
	return nil, nil
}

func (repo *MarbleDbRepository) CountDecisionsByCaseIds(ctx context.Context, tx Executor, caseIds []string) (map[string]int, error) {
	return nil, nil
}
