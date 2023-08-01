package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"
	"marble/marble-backend/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type dbScenarioIteration struct {
	Id                            string      `db:"id"`
	OrganizationId                string      `db:"org_id"`
	ScenarioId                    string      `db:"scenario_id"`
	Version                       pgtype.Int2 `db:"version"`
	CreatedAt                     time.Time   `db:"created_at"`
	UpdatedAt                     time.Time   `db:"updated_at"`
	ScoreReviewThreshold          pgtype.Int2 `db:"score_review_threshold"`
	ScoreRejectThreshold          pgtype.Int2 `db:"score_reject_threshold"`
	TriggerConditionAstExpression []byte      `db:"trigger_condition_ast_expression"`
	DeletedAt                     pgtype.Time `db:"deleted_at"`
	BatchTriggerSQL               string      `db:"batch_trigger_sql"`
	Schedule                      string      `db:"schedule"`
}

func (si *dbScenarioIteration) toDomain() (models.ScenarioIteration, error) {
	siDTO := models.ScenarioIteration{
		Id:         si.Id,
		ScenarioId: si.ScenarioId,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
		Body: models.ScenarioIterationBody{
			BatchTriggerSQL: si.BatchTriggerSQL,
			Schedule:        si.Schedule,
		},
	}

	if si.Version.Valid {
		version := int(si.Version.Int16)
		siDTO.Version = &version
	}
	if si.ScoreReviewThreshold.Valid {
		scoreReviewThreshold := int(si.ScoreReviewThreshold.Int16)
		siDTO.Body.ScoreReviewThreshold = &scoreReviewThreshold
	}
	if si.ScoreRejectThreshold.Valid {
		scoreRejectThreshold := int(si.ScoreRejectThreshold.Int16)
		siDTO.Body.ScoreRejectThreshold = &scoreRejectThreshold
	}

	var err error
	siDTO.Body.TriggerConditionAstExpression, err = dbmodels.AdaptSerializedAstExpression(si.TriggerConditionAstExpression)
	if err != nil {
		return siDTO, fmt.Errorf("unable to unmarshal trigger codition ast expression: %w", err)
	}

	return siDTO, nil
}

type dbCreateScenarioIteration struct {
	Id                            string  `db:"id"`
	OrganizationId                string  `db:"org_id"`
	ScenarioId                    string  `db:"scenario_id"`
	ScoreReviewThreshold          *int    `db:"score_review_threshold"`
	ScoreRejectThreshold          *int    `db:"score_reject_threshold"`
	TriggerConditionAstExpression *[]byte `db:"trigger_condition_ast_expression"`
	BatchTriggerSQL               string  `db:"batch_trigger_sql"`
	Schedule                      string  `db:"schedule"`
}

func (r *PGRepository) CreateScenarioIteration(ctx context.Context, organizationId string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error) {
	createScenarioIteration := dbCreateScenarioIteration{
		Id:             utils.NewPrimaryKey(organizationId),
		OrganizationId: organizationId,
		ScenarioId:     scenarioIteration.ScenarioId,
	}

	if scenarioIteration.Body != nil {
		createScenarioIteration.ScoreReviewThreshold = scenarioIteration.Body.ScoreReviewThreshold
		createScenarioIteration.ScoreRejectThreshold = scenarioIteration.Body.ScoreRejectThreshold
		createScenarioIteration.BatchTriggerSQL = scenarioIteration.Body.BatchTriggerSQL
		createScenarioIteration.Schedule = scenarioIteration.Body.Schedule

		var err error
		createScenarioIteration.TriggerConditionAstExpression, err = dbmodels.SerializeFormulaAstExpression(scenarioIteration.Body.TriggerConditionAstExpression)
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
		}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call even if tx commits

	sql, args, err := r.queryBuilder.
		Insert("scenario_iterations").
		SetMap(ColumnValueMap(createScenarioIteration)).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	createdScenarioIteration, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioIteration])
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration: %w", err)
	}

	scenarioIterationDTO, err := createdScenarioIteration.toDomain()
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
	}

	if scenarioIteration.Body != nil {
		createdRules, err := r.createScenarioIterationRules(ctx, tx, organizationId, createdScenarioIteration.Id, scenarioIteration.Body.Rules)
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf("unable to create scenario iteration rules: %w", err)
		}
		scenarioIterationDTO.Body.Rules = createdRules
	}

	err = tx.Commit(ctx)
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("transaction issue: %w", err)
	}

	return scenarioIterationDTO, nil
}

type dbUpdateScenarioIterationInput struct {
	ScoreReviewThreshold          *int    `db:"score_review_threshold"`
	ScoreRejectThreshold          *int    `db:"score_reject_threshold"`
	TriggerConditionAstExpression *[]byte `db:"trigger_condition_ast_expression"`
	BatchTriggerSQL               *string `db:"batch_trigger_sql"`
	Schedule                      *string `db:"schedule"`
}

func (r *PGRepository) UpdateScenarioIteration(ctx context.Context, organizationId string, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error) {
	if scenarioIteration.Body == nil {
		return models.ScenarioIteration{}, fmt.Errorf("nothing to update")
	}
	updateScenarioIterationInput := dbUpdateScenarioIterationInput{
		ScoreReviewThreshold: scenarioIteration.Body.ScoreReviewThreshold,
		ScoreRejectThreshold: scenarioIteration.Body.ScoreRejectThreshold,
	}
	updateScenarioIterationInput.BatchTriggerSQL = scenarioIteration.Body.BatchTriggerSQL
	updateScenarioIterationInput.Schedule = scenarioIteration.Body.Schedule

	var err error
	updateScenarioIterationInput.TriggerConditionAstExpression, err = dbmodels.SerializeFormulaAstExpression(scenarioIteration.Body.TriggerConditionAstExpression)
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call even if tx commits

	sql, args, err := r.queryBuilder.
		Select("version IS NULL").
		From("scenario_iterations").
		Where("id = ?", scenarioIteration.Id).
		Where("org_id = ?", organizationId).
		ToSql()
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	var isDraft bool
	err = tx.QueryRow(ctx, sql, args...).Scan(&isDraft)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ScenarioIteration{}, models.NotFoundInRepositoryError
	} else if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to check if scenario iteration is draft: %w", err)
	}

	if !isDraft {
		return models.ScenarioIteration{}, models.ErrScenarioIterationNotDraft
	}

	sql, args, err = r.queryBuilder.
		Update("scenario_iterations").
		SetMap(ColumnValueMap(updateScenarioIterationInput)).
		Where("id = ?", scenarioIteration.Id).
		Where("org_id = ?", organizationId).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	updatedScenarioIteration, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioIteration])
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to update scenario iteration: %w", err)
	}

	scenarioIterationDTO, err := updatedScenarioIteration.toDomain()
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("transaction issue: %w", err)
	}

	return scenarioIterationDTO, nil
}
