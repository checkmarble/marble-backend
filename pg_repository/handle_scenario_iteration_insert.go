package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"

	"github.com/jackc/pgx/v5"
)

func (r *PGRepository) CreateScenarioIteration(orgID string, scenarioIteration app.ScenarioIteration) (app.ScenarioIteration, error) {

	// scenarioIteration triggerCondition needs to be marshalled to JSON to be stored
	triggerConditionBytes, err := scenarioIteration.Body.TriggerCondition.MarshalJSON()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
	}

	// Build query
	sql, args, err := r.queryBuilder.Insert("scenario_iterations").
		Columns("org_id", "scenario_id", "version", "trigger_condition", "score_review_threshold", "score_reject_threshold").
		Values(orgID,
			scenarioIteration.ScenarioID,
			r.queryBuilder.Select("MAX(version)").From("scenario_iterations").Where("scenario_id = ?", scenarioIteration.ScenarioID),
			triggerConditionBytes,
			scenarioIteration.Body.ScoreReviewThreshold,
			scenarioIteration.Body.ScoreRejectThreshold,
		).
		Suffix("RETURNING *").ToSql()

	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	// Start the transaction
	tx, err := r.db.Begin(context.Background())
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(context.Background()) // safe to call even if tx commits

	///////////////////////////////
	// Insert scenario iteration
	///////////////////////////////

	// type used for return value
	type DBScenarioIterationRow struct {
		ID         string `db:"id"`
		ScenarioID string `db:"scenario_id"`
		Version    int    `db:"version"`

		ScoreReviewThreshold int `db:"si.score_review_threshold"`
		ScoreRejectThreshold int `db:"si.score_reject_threshold"`

		TriggerCondition []byte `db:"trigger_condition"`
	}

	// Execute query
	createdScenarioIterationRow, err := tx.Query(context.TODO(), sql, args...)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to run scenario iteration query: %w", err)
	}

	// Scan result in type
	createdScenarioIteration, err := pgx.CollectOneRow(createdScenarioIterationRow, pgx.RowToStructByName[DBScenarioIterationRow])
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to collect scenario iteration: %w", err)
	}

	createdScenarioIterationID := createdScenarioIteration.ID

	///////////////////////////////
	// Insert multiple rules in a single run
	///////////////////////////////

	// Build the base query = only columns
	query := r.queryBuilder.Insert("scenario_iteration_rules").
		Columns("org_id", "scenario_iteration_id", "display_order", "name", "description", "formula", "score_modifier")

	// Loop over rules
	for _, rule := range scenarioIteration.Body.Rules {

		// each rule's formula is marshalled into JSON to be stored
		formulaBytes, err := rule.Formula.MarshalJSON()
		if err != nil {
			return app.ScenarioIteration{}, fmt.Errorf("unable to marshal rule formula: %w", err)
		}

		// append all values to the query
		query = query.
			Values(orgID,
				createdScenarioIterationID,
				rule.DisplayOrder,
				rule.Name,
				rule.Description,
				formulaBytes,
				rule.ScoreModifier,
			)

	}

	// build query
	sql, args, err = query.
		Suffix("RETURNING *").ToSql()

	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to build rule query: %w", err)
	}

	createdScenarioIterationRuleRows, err := tx.Query(context.TODO(), sql, args...)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to run rule query: %w", err)
	}

	// type used for return value
	type DBScenarioIterationRuleRow struct {
		ScenarioIterationID string `db:"scenario_iteration_id"`
		DisplayOrder        int    `db:"display_order"`
		Name                string `db:"name"`
		Description         string `db:"description"`

		Formula       []byte `db:"formula"`
		ScoreModifier int    `db:"score_modifier"`
	}

	createdScenarioIterationRules, err := pgx.CollectRows(createdScenarioIterationRuleRows, pgx.RowToStructByName[DBScenarioIterationRuleRow])

	if len(createdScenarioIterationRules) == 0 {
		return app.ScenarioIteration{}, app.ErrNotFoundInRepository
	}
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to collect rule: %w", err)
	}

	///////////////////////////////
	// Build results
	///////////////////////////////

	// Rules
	rules := make([]app.Rule, 0)
	for _, row := range createdScenarioIterationRules {

		formula, err := operators.UnmarshalOperatorBool(row.Formula)
		if err != nil {
			return app.ScenarioIteration{}, fmt.Errorf("unable to unmarshal rule: %w", err)
		}

		rules = append(rules, app.Rule{
			DisplayOrder: row.DisplayOrder,
			Name:         row.Name,
			Description:  row.Description,

			Formula:       formula,
			ScoreModifier: row.ScoreModifier,
		})
	}

	// ScenarioIteration
	triggerc, err := operators.UnmarshalOperatorBool(createdScenarioIteration.TriggerCondition)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to unmarshal trigger condition: %w", err)
	}

	si := app.ScenarioIteration{
		ID:         createdScenarioIteration.ID,
		ScenarioID: createdScenarioIteration.ScenarioID,
		Version:    createdScenarioIteration.Version,

		// CreatedAt time.Time
		// UpdatedAt time.Time

		Body: app.ScenarioIterationBody{
			TriggerCondition:     triggerc,
			Rules:                rules,
			ScoreReviewThreshold: createdScenarioIteration.ScoreReviewThreshold,
			ScoreRejectThreshold: createdScenarioIteration.ScoreRejectThreshold,
		},
	}

	return si, nil
}
