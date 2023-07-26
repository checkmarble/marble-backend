package pg_repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/operators"
	"marble/marble-backend/repositories/dbmodels"
	"marble/marble-backend/utils"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrAlreadyPublished = errors.New("scenario iteration is already published")

type dbScenarioIteration struct {
	ID                            string          `db:"id"`
	OrgID                         string          `db:"org_id"`
	ScenarioID                    string          `db:"scenario_id"`
	Version                       pgtype.Int2     `db:"version"`
	CreatedAt                     time.Time       `db:"created_at"`
	UpdatedAt                     time.Time       `db:"updated_at"`
	ScoreReviewThreshold          pgtype.Int2     `db:"score_review_threshold"`
	ScoreRejectThreshold          pgtype.Int2     `db:"score_reject_threshold"`
	TriggerCondition              json.RawMessage `db:"trigger_condition"`
	TriggerConditionAstExpression []byte          `db:"trigger_condition_ast_expression"`
	DeletedAt                     pgtype.Time     `db:"deleted_at"`
	BatchTriggerSQL               string          `db:"batch_trigger_sql"`
	Schedule                      string          `db:"schedule"`
}

func (si *dbScenarioIteration) toDomain() (models.ScenarioIteration, error) {
	siDTO := models.ScenarioIteration{
		ID:         si.ID,
		ScenarioID: si.ScenarioID,
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
	if si.TriggerCondition != nil {
		triggerc, err := operators.UnmarshalOperatorBool(si.TriggerCondition)
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf("unable to unmarshal trigger condition: %w", err)
		}
		siDTO.Body.TriggerCondition = triggerc
	}

	var err error
	siDTO.Body.TriggerConditionAstExpression, err = dbmodels.AdaptSerizedAstExpression(si.TriggerConditionAstExpression)
	if err != nil {
		return siDTO, fmt.Errorf("unable to unmarshal trigger codition ast expression: %w", err)
	}

	return siDTO, nil
}

type ListScenarioIterationsFilters struct {
	ScenarioID *string `db:"scenario_id"`
}

func (r *PGRepository) ListScenarioIterations(ctx context.Context, orgID string, filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error) {
	sql, args, err := r.queryBuilder.
		Select(utils.ColumnList[dbScenarioIteration]()...).
		From("scenario_iterations").
		Where("org_id = ?", orgID).
		Where(sq.Eq(ColumnValueMap(ListScenarioIterationsFilters{
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

	var scenarioIterationDTOs []models.ScenarioIteration
	for _, si := range scenarioIterations {
		siDTO, err := si.toDomain()
		if err != nil {
			return nil, fmt.Errorf("dto issue: %w", err)
		}
		scenarioIterationDTOs = append(scenarioIterationDTOs, siDTO)
	}

	return scenarioIterationDTOs, nil
}

func (r *PGRepository) GetScenarioIteration(ctx context.Context, orgID string, scenarioIterationID string) (models.ScenarioIteration, error) {
	return r.getScenarioIterationRaw(ctx, r.db, orgID, scenarioIterationID)
}

func (r *PGRepository) getScenarioIterationRaw(ctx context.Context, pool PgxPoolOrTxIface, orgID string, scenarioIterationID string) (models.ScenarioIteration, error) {
	siCols := utils.ColumnList[dbScenarioIteration]("si")
	sirCols := utils.ColumnList[dbmodels.DBRule]("sir")

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
		return models.ScenarioIteration{}, fmt.Errorf("unable to build scenario iteration query: %w", err)
	}

	type DBRow struct {
		dbScenarioIteration
		Rules []dbmodels.DBRule
	}

	rows, _ := pool.Query(ctx, sql, args...)
	scenarioIteration, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[DBRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ScenarioIteration{}, models.NotFoundInRepositoryError
	} else if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to collect scenario iteration: %w", err)
	}

	scenarioIterationDTO, err := scenarioIteration.toDomain()
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
	}
	for _, rule := range scenarioIteration.Rules {
		ruleDto, err := dbmodels.AdaptRule(rule)
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf("dto issue: %w", err)
		}
		scenarioIterationDTO.Body.Rules = append(scenarioIterationDTO.Body.Rules, ruleDto)
	}

	return scenarioIterationDTO, nil
}

type dbCreateScenarioIteration struct {
	Id                   string `db:"id"`
	OrgID                string `db:"org_id"`
	ScenarioID           string `db:"scenario_id"`
	ScoreReviewThreshold *int   `db:"score_review_threshold"`
	ScoreRejectThreshold *int   `db:"score_reject_threshold"`
	TriggerCondition     []byte `db:"trigger_condition"`
	BatchTriggerSQL      string `db:"batch_trigger_sql"`
	Schedule             string `db:"schedule"`
}

func (r *PGRepository) CreateScenarioIteration(ctx context.Context, orgID string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error) {
	createScenarioIteration := dbCreateScenarioIteration{
		Id:         utils.NewPrimaryKey(orgID),
		OrgID:      orgID,
		ScenarioID: scenarioIteration.ScenarioID,
	}

	if scenarioIteration.Body != nil {
		createScenarioIteration.ScoreReviewThreshold = scenarioIteration.Body.ScoreReviewThreshold
		createScenarioIteration.ScoreRejectThreshold = scenarioIteration.Body.ScoreRejectThreshold
		createScenarioIteration.BatchTriggerSQL = scenarioIteration.Body.BatchTriggerSQL
		createScenarioIteration.Schedule = scenarioIteration.Body.Schedule

		if scenarioIteration.Body.TriggerCondition != nil {
			triggerConditionBytes, err := scenarioIteration.Body.TriggerCondition.MarshalJSON()
			if err != nil {
				return models.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
			}
			createScenarioIteration.TriggerCondition = triggerConditionBytes
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
		createdRules, err := r.createScenarioIterationRules(ctx, tx, orgID, createdScenarioIteration.ID, scenarioIteration.Body.Rules)
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
	ScoreReviewThreshold *int    `db:"score_review_threshold"`
	ScoreRejectThreshold *int    `db:"score_reject_threshold"`
	TriggerCondition     *[]byte `db:"trigger_condition"`
	BatchTriggerSQL      *string `db:"batch_trigger_sql"`
	Schedule             *string `db:"schedule"`
}

func (r *PGRepository) UpdateScenarioIteration(ctx context.Context, orgID string, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error) {
	if scenarioIteration.Body == nil {
		return models.ScenarioIteration{}, fmt.Errorf("nothing to update")
	}
	updateScenarioIterationInput := dbUpdateScenarioIterationInput{
		ScoreReviewThreshold: scenarioIteration.Body.ScoreReviewThreshold,
		ScoreRejectThreshold: scenarioIteration.Body.ScoreRejectThreshold,
	}
	updateScenarioIterationInput.BatchTriggerSQL = scenarioIteration.Body.BatchTriggerSQL
	updateScenarioIterationInput.Schedule = scenarioIteration.Body.Schedule
	if scenarioIteration.Body.TriggerCondition != nil {
		triggerConditionBytes, err := scenarioIteration.Body.TriggerCondition.MarshalJSON()
		if err != nil {
			return models.ScenarioIteration{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
		}
		updateScenarioIterationInput.TriggerCondition = &triggerConditionBytes
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return models.ScenarioIteration{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call even if tx commits

	sql, args, err := r.queryBuilder.
		Select("version IS NULL").
		From("scenario_iterations").
		Where("id = ?", scenarioIteration.ID).
		Where("org_id = ?", orgID).
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
		Where("id = ?", scenarioIteration.ID).
		Where("org_id = ?", orgID).
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
