package dbmodels

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

const TABLE_SCENARIO_ITERATIONS = "scenario_iterations"

type DBScenarioIteration struct {
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

type DBScenarioIterationWithRules struct {
	DBScenarioIteration
	Rules []DBRule `db:"rules"`
}

var SelectScenarioIterationColumn = utils.ColumnList[DBScenarioIteration]()

func AdaptScenarioIteration(dto DBScenarioIteration) (models.ScenarioIteration, error) {
	scenarioIteration := models.ScenarioIteration{
		Id:              dto.Id,
		OrganizationId:  dto.OrganizationId,
		ScenarioId:      dto.ScenarioId,
		CreatedAt:       dto.CreatedAt,
		UpdatedAt:       dto.UpdatedAt,
		BatchTriggerSQL: dto.BatchTriggerSQL,
		Schedule:        dto.Schedule,
	}

	if dto.Version.Valid {
		version := int(dto.Version.Int16)
		scenarioIteration.Version = &version
	}
	if dto.ScoreReviewThreshold.Valid {
		scoreReviewThreshold := int(dto.ScoreReviewThreshold.Int16)
		scenarioIteration.ScoreReviewThreshold = &scoreReviewThreshold
	}
	if dto.ScoreRejectThreshold.Valid {
		scoreRejectThreshold := int(dto.ScoreRejectThreshold.Int16)
		scenarioIteration.ScoreRejectThreshold = &scoreRejectThreshold
	}

	var err error
	scenarioIteration.TriggerConditionAstExpression, err =
		AdaptSerializedAstExpression(dto.TriggerConditionAstExpression)
	if err != nil {
		return scenarioIteration, fmt.Errorf("unable to unmarshal trigger codition ast expression: %w", err)
	}

	return scenarioIteration, nil
}

func AdaptScenarioIterationWithRules(dto DBScenarioIterationWithRules) (models.ScenarioIteration, error) {
	scenarioIteration, err := AdaptScenarioIteration(dto.DBScenarioIteration)
	if err != nil {
		return models.ScenarioIteration{}, err
	}

	scenarioIteration.Rules, err = pure_utils.MapErr(dto.Rules, AdaptRule)
	if err != nil {
		return models.ScenarioIteration{}, err
	}

	return scenarioIteration, nil
}
