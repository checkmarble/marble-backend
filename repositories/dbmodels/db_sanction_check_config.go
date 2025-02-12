package dbmodels

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

const TABLE_SANCTION_CHECK_CONFIGS = "sanction_check_configs"

type DBSanctionCheckConfigs struct {
	Id                  string                      `db:"id"`
	ScenarioIterationId string                      `db:"scenario_iteration_id"`
	Name                string                      `db:"name"`
	Description         string                      `db:"description"`
	RuleGroup           string                      `db:"rule_group"`
	Datasets            []string                    `db:"datasets"`
	TriggerRule         []byte                      `db:"trigger_rule"`
	Query               *DBSanctionCheckConfigQuery `db:"query"`
	ForcedOutcome       *string                     `db:"forced_outcome"`
	ScoreModifier       int                         `db:"score_modifier"`
	CounterpartyIdExpr  []byte                      `db:"counterparty_id_expression"`
	UpdatedAt           time.Time                   `db:"updated_at"`
}

type DBSanctionCheckConfigQuery struct {
	Name  json.RawMessage `json:"name"`
	Label json.RawMessage `json:"label"`
}

type DBSanctionCheckConfigQueryInput struct {
	Name  dto.NodeDto `json:"name"`
	Label dto.NodeDto `json:"label"`
}

var SanctionCheckConfigColumnList = utils.ColumnList[DBSanctionCheckConfigs]()

func AdaptSanctionCheckConfig(db DBSanctionCheckConfigs) (models.SanctionCheckConfig, error) {
	scc := models.SanctionCheckConfig{
		Name:        db.Name,
		Description: db.Description,
		RuleGroup:   &db.RuleGroup,
		Datasets:    db.Datasets,
		Outcome: models.SanctionCheckOutcome{
			ForceOutcome:  models.UnsetForcedOutcome,
			ScoreModifier: db.ScoreModifier,
		},
	}

	if db.TriggerRule != nil {
		triggerRuleAst, err := AdaptSerializedAstExpression(db.TriggerRule)
		if err != nil {
			return models.SanctionCheckConfig{}, fmt.Errorf(
				"unable to unmarshal formula ast expression: %w", err)
		}

		scc.TriggerRule = triggerRuleAst
	}

	if db.Query != nil {
		query, err := AdaptSanctionCheckConfigQuery(*db.Query)
		if err != nil {
			return models.SanctionCheckConfig{}, errors.Wrap(err, "unable to unmarshal query formula")
		}

		scc.Query = &query
	}

	if db.CounterpartyIdExpr != nil {
		field, err := AdaptSerializedAstExpression(db.CounterpartyIdExpr)
		if err != nil {
			return models.SanctionCheckConfig{}, errors.Wrap(err,
				"could not unmarshal whitelist field expression")
		}

		scc.CounterpartyIdExpression = field
	}

	if db.ForcedOutcome != nil {
		scc.Outcome.ForceOutcome = models.OutcomeFrom(*db.ForcedOutcome)
	}

	return scc, nil
}

func AdaptSanctionCheckConfigQuery(db DBSanctionCheckConfigQuery) (models.SanctionCheckConfigQuery, error) {
	nameAst, err := AdaptSerializedAstExpression(db.Name)
	if err != nil {
		return models.SanctionCheckConfigQuery{}, err
	}
	labelAst, err := AdaptSerializedAstExpression(db.Label)
	if err != nil {
		return models.SanctionCheckConfigQuery{}, err
	}

	model := models.SanctionCheckConfigQuery{
		Name:  *nameAst,
		Label: labelAst,
	}

	return model, nil
}
