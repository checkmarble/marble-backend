package pg_repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type dbScenarioIteration struct {
	ID                   string          `db:"id"`
	OrgID                string          `db:"org_id"`
	ScenarioID           string          `db:"scenario_id"`
	Version              int             `db:"version"`
	CreatedAt            time.Time       `db:"created_at"`
	UpdatedAt            time.Time       `db:"updated_at"`
	ScoreReviewThreshold pgtype.Int2     `db:"score_review_threshold"`
	ScoreRejectThreshold pgtype.Int2     `db:"score_reject_threshold"`
	TriggerCondition     json.RawMessage `db:"trigger_condition"`
	DeletedAt            pgtype.Time     `db:"deleted_at"`
}

func (si *dbScenarioIteration) dto() (app.ScenarioIteration, error) {
	siDTO := app.ScenarioIteration{
		ID:         si.ID,
		ScenarioID: si.ScenarioID,
		Version:    si.Version,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
	}

	if si.ScoreReviewThreshold.Valid {
		siDTO.Body.ScoreReviewThreshold = int(si.ScoreReviewThreshold.Int16)
	}
	if si.ScoreRejectThreshold.Valid {
		siDTO.Body.ScoreRejectThreshold = int(si.ScoreRejectThreshold.Int16)
	}
	if si.TriggerCondition != nil {
		triggerc, err := operators.UnmarshalOperatorBool(si.TriggerCondition)
		if err != nil {
			return app.ScenarioIteration{}, fmt.Errorf("unable to unmarshal trigger condition: %w", err)
		}
		siDTO.Body.TriggerCondition = triggerc
	}

	return siDTO, nil
}

type ListScenarioIterationsFilters struct {
	ScenarioID *string `db:"scenario_id"`
}

func (r *PGRepository) ListScenarioIterations(ctx context.Context, orgID string, filters app.GetScenarioIterationFilters) ([]app.ScenarioIteration, error) {
	sql, args, err := r.queryBuilder.
		Select(columnList[dbScenarioIteration]()...).
		From("scenario_iterations").
		Where("org_id = ?", orgID).
		Where(squirrel.Eq(columnValueMap(ListScenarioIterationsFilters{
			ScenarioID: filters.ScenarioID,
		}))).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenarioIterations, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbScenarioIteration])
	if err != nil {
		return nil, fmt.Errorf("unable to collect scenario iteration: %w", err)
	}

	var scenarioIterationDTOs []app.ScenarioIteration
	for _, si := range scenarioIterations {
		siDTO, err := si.dto()
		if err != nil {
			return nil, fmt.Errorf("dto issue: %w", err)
		}
		scenarioIterationDTOs = append(scenarioIterationDTOs, siDTO)
	}

	return scenarioIterationDTOs, nil
}

func (r *PGRepository) GetScenarioIteration(ctx context.Context, orgID string, scenarioIterationID string) (app.ScenarioIteration, error) {
	siCols := columnList[dbScenarioIteration]("si")
	sirCols := columnList[dbScenarioIterationRule]("sir")

	sql, args, err := r.queryBuilder.
		Select(siCols...).
		Column(fmt.Sprintf("array_agg(row(%s)) FILTER (WHERE sir.id IS NOT NULL) as rules", strings.Join(sirCols, ","))).
		From("scenario_iterations si").
		LeftJoin("scenario_iteration_rules sir on sir.scenario_iteration_id = si.id").
		Where("si.id = ?", scenarioIterationID).
		Where("si.org_id = ?", orgID).
		GroupBy("si.id").
		ToSql()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	type DBRow struct {
		dbScenarioIteration
		Rules []dbScenarioIterationRule
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenarioIteration, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[DBRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ScenarioIteration{}, app.ErrNotFoundInRepository
	} else if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to collect scenario iteration: %w", err)
	}

	scenarioIterationDTO, err := scenarioIteration.dto()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
	}
	for _, rule := range scenarioIteration.Rules {
		ruleDto, err := rule.dto()
		if err != nil {
			return app.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
		}
		scenarioIterationDTO.Body.Rules = append(scenarioIterationDTO.Body.Rules, ruleDto)
	}

	return scenarioIterationDTO, nil
}

type dbCreateScenarioIteration struct {
	OrgID                string `db:"org_id"`
	ScenarioID           string `db:"scenario_id"`
	Version              int    `db:"version"`
	ScoreReviewThreshold *int   `db:"score_review_threshold"`
	ScoreRejectThreshold *int   `db:"score_reject_threshold"`
	TriggerCondition     []byte `db:"trigger_condition"`
}

func (r *PGRepository) CreateScenarioIteration(ctx context.Context, orgID string, scenarioIteration app.CreateScenarioIterationInput) (app.ScenarioIteration, error) {
	createScenarioIteration := dbCreateScenarioIteration{
		OrgID:      orgID,
		ScenarioID: scenarioIteration.ScenarioID,
	}

	if scenarioIteration.Body != nil {
		createScenarioIteration.ScoreReviewThreshold = scenarioIteration.Body.ScoreReviewThreshold
		createScenarioIteration.ScoreRejectThreshold = scenarioIteration.Body.ScoreRejectThreshold

		if scenarioIteration.Body.TriggerCondition != nil {
			triggerConditionBytes, err := scenarioIteration.Body.TriggerCondition.MarshalJSON()
			if err != nil {
				return app.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
			}
			createScenarioIteration.TriggerCondition = triggerConditionBytes
		}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call even if tx commits

	sql, args, err := r.queryBuilder.
		Select("COALESCE(MAX(version)+1, 1)").
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
	createScenarioIteration.Version = version

	sql, args, err = r.queryBuilder.
		Insert("scenario_iterations").
		SetMap(columnValueMap(createScenarioIteration)).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	createdScenarioIteration, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioIteration])
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration: %w", err)
	}

	scenarioIterationDTO, err := createdScenarioIteration.dto()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
	}

	if scenarioIteration.Body != nil {
		createdRules, err := r.createScenarioIterationRules(ctx, tx, orgID, createdScenarioIteration.ID, scenarioIteration.Body.Rules)
		if err != nil {
			return app.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration rules: %w", err)
		}
		scenarioIterationDTO.Body.Rules = createdRules
	}

	tx.Commit(ctx)

	return scenarioIterationDTO, nil
}

type dbUpdateScenarioIterationInput struct {
	ScoreReviewThreshold *int    `db:"score_review_threshold"`
	ScoreRejectThreshold *int    `db:"score_reject_threshold"`
	TriggerCondition     *[]byte `db:"trigger_condition"`
}

func (r *PGRepository) UpdateScenarioIteration(ctx context.Context, orgID string, scenarioIteration app.UpdateScenarioIterationInput) (app.ScenarioIteration, error) {
	if scenarioIteration.Body == nil {
		return app.ScenarioIteration{}, fmt.Errorf("nothing to update")
	}
	updateScenarioIterationInput := dbUpdateScenarioIterationInput{
		ScoreReviewThreshold: scenarioIteration.Body.ScoreReviewThreshold,
		ScoreRejectThreshold: scenarioIteration.Body.ScoreRejectThreshold,
	}
	if scenarioIteration.Body.TriggerCondition != nil {
		triggerConditionBytes, err := scenarioIteration.Body.TriggerCondition.MarshalJSON()
		if err != nil {
			return app.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
		}
		updateScenarioIterationInput.TriggerCondition = &triggerConditionBytes
	}

	sql, args, err := r.queryBuilder.
		Update("scenario_iterations").
		SetMap(columnValueMap(updateScenarioIterationInput)).
		Where("id = ?", scenarioIteration.ID).
		Where("org_id = ?", orgID).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	updatedScenarioIteration, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioIteration])
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("unable to update scenario iteration: %w", err)
	}

	scenarioIterationDTO, err := updatedScenarioIteration.dto()
	if err != nil {
		return app.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
	}

	return scenarioIterationDTO, nil
}
