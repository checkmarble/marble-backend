package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type dbScenarioIteration struct {
	ID                   string    `db:"id"`
	ScenarioID           string    `db:"scenario_id"`
	Version              int       `db:"version"`
	CreatedAt            time.Time `db:"created_at"`
	UpdatedAt            time.Time `db:"updated_at"`
	ScoreReviewThreshold int       `db:"si.score_review_threshold"`
	ScoreRejectThreshold int       `db:"si.score_reject_threshold"`
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

func (r *PGRepository) getNextVersionNumberBuilder(scenarioID string) squirrel.SelectBuilder {
	return r.queryBuilder.Select("MAX(version)").From("scenario_iterations").Where("scenario_id = ?", scenarioID)
}

func (r *PGRepository) CreateScenarioIteration(orgID string, scenarioIteration app.ScenarioIteration) (app.ScenarioIteration, error) {
	triggerConditionBytes, err := scenarioIteration.Body.TriggerCondition.MarshalJSON()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
	}

	sql, args, err := r.queryBuilder.
		Insert("scenario_iterations").
		Columns(
			"org_id",
			"scenario_id",
			"version",
			"trigger_condition",
			"score_review_threshold",
			"score_reject_threshold",
		).
		Values(
			orgID,
			scenarioIteration.ScenarioID,
			r.getNextVersionNumberBuilder(scenarioIteration.ScenarioID),
			triggerConditionBytes,
			scenarioIteration.Body.ScoreReviewThreshold,
			scenarioIteration.Body.ScoreRejectThreshold,
		).
		Suffix("RETURNING *").ToSql()

	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	tx, err := r.db.Begin(context.Background())
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(context.Background()) // safe to call even if tx commits

	rows, _ := tx.Query(context.TODO(), sql, args...)
	createdScenarioIteration, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioIteration])
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration: %w", err)
	}

	createdRules, err := r.CreateScenarioIterationRules(createdScenarioIteration.ID, scenarioIteration.Body.Rules)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration rules: %w", err)
	}

	scenarioIterationDTO, err := createdScenarioIteration.dto()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
	}

	scenarioIterationDTO.Body.Rules = createdRules

	return scenarioIterationDTO, nil
}
