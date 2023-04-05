package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"
	"time"

	"github.com/jackc/pgx/v5"
)

type dbScenarioIteration struct {
	ID                   string    `db:"id"`
	OrgID                string    `db:"org_id"`
	ScenarioID           string    `db:"scenario_id"`
	Version              int       `db:"version"`
	CreatedAt            time.Time `db:"created_at"`
	UpdatedAt            time.Time `db:"updated_at"`
	ScoreReviewThreshold int       `db:"score_review_threshold"`
	ScoreRejectThreshold int       `db:"score_reject_threshold"`
	TriggerCondition     []byte    `db:"trigger_condition"`
}

func (si *dbScenarioIteration) dto() (app.ScenarioIteration, error) {
	triggerc, err := operators.UnmarshalOperatorBool(si.TriggerCondition)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to unmarshal trigger condition: %w", err)
	}

	return app.ScenarioIteration{
		ID:         si.ID,
		ScenarioID: si.ScenarioID,
		Version:    si.Version,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
		Body: app.ScenarioIterationBody{
			TriggerCondition:     triggerc,
			ScoreReviewThreshold: si.ScoreReviewThreshold,
			ScoreRejectThreshold: si.ScoreRejectThreshold,
		},
	}, nil
}

func (r *PGRepository) getNextVersionNumberBuilder(scenarioID string) (string, []interface{}, error) {
	return r.queryBuilder.
		Select("MAX(version)+1 as version").
		From("scenario_iterations").
		Where("scenario_id = ?", scenarioID).ToSql()
}

func (r *PGRepository) CreateScenarioIteration(ctx context.Context, orgID string, scenarioIteration app.ScenarioIteration) (app.ScenarioIteration, error) {
	triggerConditionBytes, err := scenarioIteration.Body.TriggerCondition.MarshalJSON()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call even if tx commits

	sql, args, err := r.queryBuilder.
		Select("MAX(version)+1").
		From("scenario_iterations").
		Where("scenario_id = ?", scenarioIteration.ScenarioID).ToSql()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to build next iteration version query: %w", err)
	}

	var version int
	err = tx.QueryRow(ctx, sql, args...).Scan(&version)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to get scenario next iteration version: %w", err)
	}

	sql, args, err = r.queryBuilder.
		Insert("scenario_iterations").
		Columns(
			"scenario_id",
			"org_id",
			"version",
			"trigger_condition",
			"score_review_threshold",
			"score_reject_threshold",
		).
		Values(
			scenarioIteration.ScenarioID,
			orgID,
			version,
			string(triggerConditionBytes),
			scenarioIteration.Body.ScoreReviewThreshold,
			scenarioIteration.Body.ScoreRejectThreshold,
		).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	createdScenarioIteration, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioIteration])
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration: %w", err)
	}

	createdRules, err := r.createScenarioIterationRules(ctx, tx, orgID, createdScenarioIteration.ID, scenarioIteration.Body.Rules)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration rules: %w", err)
	}

	scenarioIterationDTO, err := createdScenarioIteration.dto()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
	}

	scenarioIterationDTO.Body.Rules = createdRules

	tx.Commit(ctx)

	return scenarioIterationDTO, nil
}
