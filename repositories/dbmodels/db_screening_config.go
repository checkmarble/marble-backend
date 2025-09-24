package dbmodels

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

const TABLE_SCREENING_CONFIGS = "screening_configs"

type DBScreeningConfigs struct {
	Id                  string                              `db:"id"`
	StableId            string                              `db:"stable_id"`
	ScenarioIterationId string                              `db:"scenario_iteration_id"`
	Name                string                              `db:"name"`
	Description         string                              `db:"description"`
	RuleGroup           string                              `db:"rule_group"`
	Datasets            []string                            `db:"datasets"`
	TriggerRule         []byte                              `db:"trigger_rule"`
	EntityType          string                              `db:"entity_type"`
	Query               json.RawMessage                     `db:"query"`
	Threshold           *int                                `db:"threshold"`
	ForcedOutcome       string                              `db:"forced_outcome"`
	CounterpartyIdExpr  []byte                              `db:"counterparty_id_expression"`
	UpdatedAt           time.Time                           `db:"updated_at"`
	Preprocessing       models.ScreeningConfigPreprocessing `db:"preprocessing"`
	ConfigVersion       string                              `db:"config_version"`
}

var ScreeningConfigColumnList = utils.ColumnList[DBScreeningConfigs]()

func AdaptScreeningConfig(db DBScreeningConfigs) (models.ScreeningConfig, error) {
	scc := models.ScreeningConfig{
		Id:                  db.Id,
		ScenarioIterationId: db.ScenarioIterationId,
		StableId:            db.StableId,
		Name:                db.Name,
		Description:         db.Description,
		RuleGroup:           &db.RuleGroup,
		EntityType:          db.EntityType,
		Datasets:            db.Datasets,
		Threshold:           db.Threshold,
		ForcedOutcome:       models.OutcomeFrom(db.ForcedOutcome),
		Preprocessing:       db.Preprocessing,
		ConfigVersion:       db.ConfigVersion,
	}

	if db.TriggerRule != nil {
		triggerRuleAst, err := AdaptSerializedAstExpression(db.TriggerRule)
		if err != nil {
			return models.ScreeningConfig{}, fmt.Errorf(
				"unable to unmarshal formula ast expression: %w", err)
		}

		scc.TriggerRule = triggerRuleAst
	}

	if db.Query != nil {
		query, err := AdaptScreeningConfigQuery(db.Query)
		if err != nil {
			return models.ScreeningConfig{}, errors.Wrap(err, "unable to unmarshal query formula")
		}

		scc.Query = query
	}

	if db.CounterpartyIdExpr != nil {
		field, err := AdaptSerializedAstExpression(db.CounterpartyIdExpr)
		if err != nil {
			return models.ScreeningConfig{}, errors.Wrap(err,
				"could not unmarshal whitelist field expression")
		}

		scc.CounterpartyIdExpression = field
	}

	return scc, nil
}

func AdaptScreeningConfigQuery(db json.RawMessage) (map[string]ast.Node, error) {
	var anyMap map[string]json.RawMessage

	if err := json.Unmarshal(db, &anyMap); err != nil {
		return nil, err
	}

	astMap := make(map[string]ast.Node)

	for k := range anyMap {
		nameAst, err := AdaptSerializedAstExpression(anyMap[k])
		if err != nil {
			return nil, err
		}

		astMap[k] = *nameAst
	}
	return astMap, nil
}
