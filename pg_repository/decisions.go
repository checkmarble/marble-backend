package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/repositories/dbmodels"

	"marble/marble-backend/models"
)

type DbDecisionWithRules struct {
	dbmodels.DbDecision
	Rules []dbmodels.DbDecisionRule
}

func (r *PGRepository) StoreDecision(ctx context.Context, orgID string, decision models.Decision) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	sql, args, err := r.queryBuilder.
		Insert("decisions").
		Columns(
			"id",
			"org_id",
			"outcome",
			"scenario_id",
			"scenario_name",
			"scenario_description",
			"scenario_version",
			"score",
			"error_code",
			"trigger_object",
			"trigger_object_type",
		).
		Values(
			decision.DecisionId,
			orgID,
			decision.Outcome.String(),
			decision.ScenarioId,
			decision.ScenarioName,
			decision.ScenarioDescription,
			decision.ScenarioVersion,
			decision.Score,
			decision.DecisionError,
			decision.ClientObject.Data,
			decision.ClientObject.TableName,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("unable to build decision query: %w", err)
	}

	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("unable to create decision: %w", err)
	}

	err = r.createDecisionRules(ctx, tx, orgID, decision.DecisionId, decision.RuleExecutions)
	if err != nil {
		return fmt.Errorf("unable to create decision rules: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("transaction issue: %w", err)
	}

	return nil
}
