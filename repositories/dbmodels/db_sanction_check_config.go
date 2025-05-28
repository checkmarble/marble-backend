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

const TABLE_SANCTION_CHECK_CONFIGS = "sanction_check_configs"

type DBSanctionCheckConfigs struct {
	Id                  string          `db:"id"`
	StableId            string          `db:"stable_id"`
	ScenarioIterationId string          `db:"scenario_iteration_id"`
	Name                string          `db:"name"`
	Description         string          `db:"description"`
	RuleGroup           string          `db:"rule_group"`
	Datasets            []string        `db:"datasets"`
	TriggerRule         []byte          `db:"trigger_rule"`
	Query               json.RawMessage `db:"query"`
	ForcedOutcome       string          `db:"forced_outcome"`
	CounterpartyIdExpr  []byte          `db:"counterparty_id_expression"`
	UpdatedAt           time.Time       `db:"updated_at"`
	ConfigVersion       string          `db:"config_version"`
}

var SanctionCheckConfigColumnList = utils.ColumnList[DBSanctionCheckConfigs]()

func AdaptSanctionCheckConfig(db DBSanctionCheckConfigs) (models.SanctionCheckConfig, error) {
	scc := models.SanctionCheckConfig{
		Id:                  db.Id,
		ScenarioIterationId: db.ScenarioIterationId,
		StableId:            db.StableId,
		Name:                db.Name,
		Description:         db.Description,
		RuleGroup:           &db.RuleGroup,
		Datasets:            db.Datasets,
		ForcedOutcome:       models.OutcomeFrom(db.ForcedOutcome),
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
		query, err := AdaptSanctionCheckConfigQuery(db.Query)
		if err != nil {
			return models.SanctionCheckConfig{}, errors.Wrap(err, "unable to unmarshal query formula")
		}

		scc.Query = query
	}

	if db.CounterpartyIdExpr != nil {
		field, err := AdaptSerializedAstExpression(db.CounterpartyIdExpr)
		if err != nil {
			return models.SanctionCheckConfig{}, errors.Wrap(err,
				"could not unmarshal whitelist field expression")
		}

		scc.CounterpartyIdExpression = field
	}

	return scc, nil
}

func AdaptSanctionCheckConfigQuery(db json.RawMessage) (*ast.Node, error) {
	nameAst, err := AdaptSerializedAstExpression(db)
	if err != nil {
		return nil, err
	}
	return nameAst, nil
}
