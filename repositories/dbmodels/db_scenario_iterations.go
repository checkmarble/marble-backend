package dbmodels

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgtype"
)

const TABLE_SCENARIO_ITERATIONS = "scenario_iterations"

type DBScenarioIteration struct {
	Id                            string      `db:"id"`
	OrganizationId                uuid.UUID   `db:"org_id"`
	ScenarioId                    string      `db:"scenario_id"`
	Version                       pgtype.Int2 `db:"version"`
	CreatedAt                     time.Time   `db:"created_at"`
	UpdatedAt                     time.Time   `db:"updated_at"`
	ScoreReviewThreshold          pgtype.Int2 `db:"score_review_threshold"`
	ScoreBlockAndReviewThreshold  pgtype.Int2 `db:"score_block_and_review_threshold"`
	ScoreDeclineThreshold         pgtype.Int2 `db:"score_reject_threshold"` // warning: field named inconsistently
	TriggerConditionAstExpression []byte      `db:"trigger_condition_ast_expression"`
	DeletedAt                     pgtype.Time `db:"deleted_at"`
	Schedule                      string      `db:"schedule"`
	Archived                      bool        `db:"archived"`
}

type DBScenarioIterationMetadata struct {
	Id             string    `db:"id"`
	OrganizationId uuid.UUID `db:"org_id"`
	ScenarioId     string    `db:"scenario_id"`
	Version        *int      `db:"version"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Archived       bool      `db:"archived"`
}

type DBScenarioIterationWithRules struct {
	DBScenarioIteration
	Rules []DBRule `db:"rules"`
}

var (
	SelectScenarioIterationColumn         = utils.ColumnList[DBScenarioIteration]()
	SelectScenarioIterationMetadataColumn = utils.ColumnList[DBScenarioIterationMetadata]()
)

func AdaptScenarioIteration(dto DBScenarioIteration) (models.ScenarioIteration, error) {
	scenarioIteration := models.ScenarioIteration{
		Id:             dto.Id,
		OrganizationId: dto.OrganizationId,
		ScenarioId:     dto.ScenarioId,
		CreatedAt:      dto.CreatedAt,
		UpdatedAt:      dto.UpdatedAt,
		Schedule:       dto.Schedule,
		Archived:       dto.Archived,
	}

	if dto.Version.Valid {
		version := int(dto.Version.Int16)
		scenarioIteration.Version = &version
	}
	if dto.ScoreReviewThreshold.Valid {
		scoreReviewThreshold := int(dto.ScoreReviewThreshold.Int16)
		scenarioIteration.ScoreReviewThreshold = &scoreReviewThreshold
	}
	if dto.ScoreBlockAndReviewThreshold.Valid {
		scoreBlockAndReviewThreshold := int(dto.ScoreBlockAndReviewThreshold.Int16)
		scenarioIteration.ScoreBlockAndReviewThreshold = &scoreBlockAndReviewThreshold
	}
	if dto.ScoreDeclineThreshold.Valid {
		scoreDeclineThreshold := int(dto.ScoreDeclineThreshold.Int16)
		scenarioIteration.ScoreDeclineThreshold = &scoreDeclineThreshold
	}

	var err error
	scenarioIteration.TriggerConditionAstExpression, err = AdaptSerializedAstExpression(dto.TriggerConditionAstExpression)
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

func AdaptScenarioIterationMetadata(dto DBScenarioIterationMetadata) (models.ScenarioIterationMetadata, error) {
	return models.ScenarioIterationMetadata{
		Id:             dto.Id,
		OrganizationId: dto.OrganizationId,
		ScenarioId:     dto.ScenarioId,
		Version:        dto.Version,
		CreatedAt:      dto.CreatedAt,
		UpdatedAt:      dto.UpdatedAt,
		Archived:       dto.Archived,
	}, nil
}

type DBRulesAndScreenings struct {
	ScenarioIterationId      uuid.UUID       `db:"id"`
	ScenarioId               uuid.UUID       `db:"scenario_id"`
	Name                     string          `db:"name"`
	RuleId                   uuid.UUID       `db:"rule_id"`
	Version                  *int            `db:"version"`
	TriggerAst               json.RawMessage `db:"trigger_ast"`
	RuleAst                  json.RawMessage `db:"rule_ast"`
	ScreeningTriggerAst      json.RawMessage `db:"screening_trigger_ast"`
	ScreeningCounterpartyAst json.RawMessage `db:"screening_counterparty_ast"`
	ScreeningAst             json.RawMessage `db:"screening_ast"`
}

func AdaptRulesAndScreenings(db DBRulesAndScreenings) (models.RulesAndScreenings, error) {
	out := models.RulesAndScreenings{
		ScenarioIterationId: db.ScenarioIterationId,
		ScenarioId:          db.ScenarioId,
		RuleId:              db.RuleId,
		Name:                db.Name,
		Version:             db.Version,
	}

	var err error

	out.TriggerAst, err = AdaptSerializedAstExpression(db.TriggerAst)
	if err != nil {
		return out, fmt.Errorf("unable to unmarshal trigger codition ast expression: %w", err)
	}
	out.RuleAst, err = AdaptSerializedAstExpression(db.RuleAst)
	if err != nil {
		return out, fmt.Errorf("unable to unmarshal trigger codition ast expression: %w", err)
	}
	out.ScreeningTriggerAst, err = AdaptSerializedAstExpression(db.ScreeningTriggerAst)
	if err != nil {
		return out, fmt.Errorf("unable to unmarshal trigger codition ast expression: %w", err)
	}
	out.ScreeningCounterpartyAst, err = AdaptSerializedAstExpression(db.ScreeningCounterpartyAst)
	if err != nil {
		return out, fmt.Errorf("unable to unmarshal trigger codition ast expression: %w", err)
	}
	if db.ScreeningAst != nil {
		out.ScreeningAst, err = AdaptScreeningConfigQuery(db.ScreeningAst)
		if err != nil {
			return out, fmt.Errorf("unable to unmarshal trigger codition ast expression: %w", err)
		}
	}

	return out, nil
}
