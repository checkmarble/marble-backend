package dbmodels

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECK_CONFIGS = "sanction_check_configs"

type DBSanctionCheckConfigs struct {
	Id                  string    `db:"id"`
	ScenarioIterationId string    `db:"scenario_iteration_id"`
	Enabled             bool      `db:"enabled"`
	TriggerRule         []byte    `db:"trigger_rule"`
	ForcedOutcome       *string   `db:"forced_outcome"`
	ScoreModifier       int       `db:"score_modifier"`
	UpdatedAt           time.Time `db:"updated_at"`
}

var SanctionCheckConfigColumnList = utils.ColumnList[DBSanctionCheckConfigs]()

func AdaptSanctionCheckConfig(db DBSanctionCheckConfigs) (models.SanctionCheckConfig, error) {
	triggerRuleAst, err := AdaptSerializedAstExpression(db.TriggerRule)
	if err != nil {
		return models.SanctionCheckConfig{}, fmt.Errorf(
			"unable to unmarshal formula ast expression: %w", err)
	}

	var forcedOutcome models.Outcome = models.UnsetForcedOutcome

	if db.ForcedOutcome != nil {
		forcedOutcome = models.OutcomeFrom(*db.ForcedOutcome)
	}

	scc := models.SanctionCheckConfig{
		Enabled:     db.Enabled,
		TriggerRule: triggerRuleAst,
		Outcome: models.SanctionCheckOutcome{
			ForceOutcome:  forcedOutcome,
			ScoreModifier: db.ScoreModifier,
		},
	}

	return scc, nil
}
