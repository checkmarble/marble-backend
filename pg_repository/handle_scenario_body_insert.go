package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/app"
)

func (r *PGRepository) CreateScenarioIteration(orgID string, scenarioIteration app.ScenarioIteration) (id string, err error) {

	// Marshall triggerCondition
	triggerConditionBytes, err := scenarioIteration.Body.TriggerCondition.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("unable to marshal trigger condition")
	}

	///////////////////////////////
	// DB transaction
	// Store rules + body in one go
	///////////////////////////////

	// Scenario Iteration
	sql, args, err := r.queryBuilder.Insert("scenario_iterations").
		Columns("org_id", "scenario_id", "version", "trigger_condition", "score_review_threshold", "score_reject_threshold").
		Values(orgID,
			scenarioIteration.ScenarioID,
			r.queryBuilder.Select("MAX(version)").From("scenario_iterations").Where("scenario_id = ?", scenarioIteration.ScenarioID),
			triggerConditionBytes,
			scenarioIteration.Body.ScoreReviewThreshold,
			scenarioIteration.Body.ScoreRejectThreshold,
		).
		Suffix("RETURNING \"id\"").ToSql()

	if err != nil {
		return "", fmt.Errorf("unable to build a query: %w", err)
	}

	tx, err := r.db.Begin(context.Background())
	if err != nil {
		return "", fmt.Errorf("unable to start a transaction: %w", err)
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	defer tx.Rollback(context.Background())

	// Scenario Iteration

	var createdIterationID string
	err = r.db.QueryRow(context.TODO(), sql, args).Scan(&createdIterationID)

	if err != nil {
		return "", fmt.Errorf("unable to insert scenario iteration: %w", err)
	}

	// ScenarioIterationRules
	for _, rule := range scenarioIteration.Body.Rules {
		formulaBytes, err := rule.Formula.MarshalJSON()
		if err != nil {
			return "", fmt.Errorf("unable to marshal rule formula: %w", err)
		}

		// TODO(bulk insert): handle bulk insert
		sql, args, err := r.queryBuilder.Insert("scenario_iteration_rules").
			Columns("org_id", "scenario_iteration_id", "display_order", "name", "description", "formula", "score_modifier").
			Values(orgID,
				createdIterationID,
				rule.DisplayOrder,
				rule.Name,
				rule.Description,
				formulaBytes,
				rule.ScoreModifier,
			).ToSql()

		if err != nil {
			return "", fmt.Errorf("unable to build a query: %w", err)
		}

		_, err = tx.Exec(context.TODO(), sql, args)

		if err != nil {
			return "", fmt.Errorf("unable to insert rule: %w", err)
		}

	}

	return createdIterationID, nil
}
