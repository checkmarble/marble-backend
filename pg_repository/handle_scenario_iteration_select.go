package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"

	"github.com/jackc/pgx/v5"
)

func (r *PGRepository) GetScenarioIteration(orgID string, scenarioIterationID string) (app.ScenarioIteration, error) {

	///////////////////////////////
	// Query DB
	///////////////////////////////

	// Build query
	sql, args, err := r.queryBuilder.
		Select("si.id, si.scenario_id, si.version, si.score_review_threshold, si.score_reject_threshold, si.trigger_condition, sir.display_order, sir.name, sir.description, sir.formula, sir.score_modifier").
		From("scenario_iteration si").
		Join("scenario_iteration_rules sir ON sir.scenario_iteration_id = si.id").
		Where("si.id = ?", scenarioIterationID).
		Where("si.ord_id = ?", orgID).
		ToSql()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	// Execute query
	// Struct corresponding to rows
	type DBRow struct {
		ID         string `db:"si.id"`
		ScenarioID string `db:"si.scenario_id"`
		Version    int    `db:"si.version"`

		ScoreReviewThreshold int `db:"si.score_review_threshold"`
		ScoreRejectThreshold int `db:"si.score_reject_threshold"`

		TriggerCondition []byte `db:"si.trigger_condition"`

		DisplayOrder int    `db:"sir.display_order"`
		Name         string `db:"sir.name"`
		Description  string `db:"sir.formula"`

		Formula       []byte `db:"sir.formula"`
		ScoreModifier int    `db:"si.score_modifier"`
	}

	rows, err := r.db.Query(context.TODO(), sql, args...)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to run scenario iteration query: %w", err)
	}

	dbRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[DBRow])

	if errors.Is(err, pgx.ErrNoRows) {
		return app.ScenarioIteration{}, app.ErrNotFoundInRepository
	}
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to collect scenario iteration: %w", err)
	}

	///////////////////////////////
	// Build results
	///////////////////////////////

	// Rules
	rules := make([]app.Rule, 0)
	for _, row := range dbRows {

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
	triggerc, err := operators.UnmarshalOperatorBool(dbRows[0].TriggerCondition)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to unmarshal trigger condition: %w", err)
	}

	si := app.ScenarioIteration{
		ID:         dbRows[0].ID,
		ScenarioID: dbRows[0].ScenarioID,
		Version:    dbRows[0].Version,

		// CreatedAt time.Time
		// UpdatedAt time.Time

		Body: app.ScenarioIterationBody{
			TriggerCondition:     triggerc,
			Rules:                rules,
			ScoreReviewThreshold: dbRows[0].ScoreReviewThreshold,
			ScoreRejectThreshold: dbRows[0].ScoreRejectThreshold,
		},
	}

	return si, nil
}
