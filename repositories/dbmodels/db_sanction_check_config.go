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
	Id                  string                     `db:"id"`
	ScenarioIterationId string                     `db:"scenario_iteration_id"`
	Datasets            []string                   `db:"datasets"`
	TriggerRule         []byte                     `db:"trigger_rule"`
	Query               DBSanctionCheckConfigQuery `db:"query"`
	ForcedOutcome       *string                    `db:"forced_outcome"`
	ScoreModifier       int                        `db:"score_modifier"`
	UpdatedAt           time.Time                  `db:"updated_at"`
}

type DBSanctionCheckConfigQuery struct {
	Name json.RawMessage `json:"name"`
}

type DBSanctionCheckConfigQueryInput struct {
	Name dto.NodeDto `json:"name"`
}

var SanctionCheckConfigColumnList = utils.ColumnList[DBSanctionCheckConfigs]()

func AdaptSanctionCheckConfig(db DBSanctionCheckConfigs) (models.SanctionCheckConfig, error) {
	triggerRuleAst, err := AdaptSerializedAstExpression(db.TriggerRule)
	if err != nil {
		return models.SanctionCheckConfig{}, fmt.Errorf(
			"unable to unmarshal formula ast expression: %w", err)
	}

	query, err := AdaptSanctionCheckConfigQuery(db.Query)
	if err != nil {
		return models.SanctionCheckConfig{}, errors.Wrap(err, "unable to unmarshal query formula")
	}

	var forcedOutcome models.Outcome = models.UnsetForcedOutcome

	if db.ForcedOutcome != nil {
		forcedOutcome = models.OutcomeFrom(*db.ForcedOutcome)
	}

	scc := models.SanctionCheckConfig{
		Datasets:    db.Datasets,
		TriggerRule: *triggerRuleAst,
		Query:       query,
		Outcome: models.SanctionCheckOutcome{
			ForceOutcome:  forcedOutcome,
			ScoreModifier: db.ScoreModifier,
		},
	}

	return scc, nil
}

func AdaptSanctionCheckConfigQuery(db DBSanctionCheckConfigQuery) (models.SanctionCheckConfigQuery, error) {
	nameAst, err := AdaptSerializedAstExpression(db.Name)
	if err != nil {
		return models.SanctionCheckConfigQuery{}, err
	}

	model := models.SanctionCheckConfigQuery{
		Name: *nameAst,
	}

	return model, nil
}
