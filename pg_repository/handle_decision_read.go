package pg_repository

import (
	"context"
	"fmt"
	"time"

	"marble/marble-backend/app"
	"marble/marble-backend/app/scenarios"

	"github.com/jackc/pgx/v5"
)

func (r *PGRepository) GetDecision(orgID string, decisionID string) (decision scenarios.Decision, err error) {

	queryString := `
	SELECT
	d.id,
	d.created_at,
	d.outcome,
	d.scenario_id,
	d.scenario_name,
	d.scenario_description,
	d.scenario_version,
	d.score,
	d.error_code,

	dr.name,
	dr.description,
	dr.score_modifier,
	dr.result,
	dr.error_code
	
	FROM decisions d LEFT JOIN decision_rules dr ON dr.decision_id = d.id
	WHERE d.org_id = $1
	AND dr.org_id = $1

	AND d.id = $2
	`

	rows, _ := r.db.Query(context.Background(), queryString, orgID, decisionID)

	var d_id string
	var d_created_at time.Time
	var d_outcome string
	var d_scenario_id string
	var d_scenario_name string
	var d_scenario_description string
	var d_scenario_version int
	var d_score int
	var d_error_code int

	var dr_name string
	var dr_description string
	var dr_score_modifier int
	var dr_result bool
	var dr_error_code int

	// create an empty Decision object
	d := scenarios.Decision{}

	// Loop counter
	i := 0

	_, err = pgx.ForEachRow(rows, []any{&d_id, &d_created_at, &d_outcome, &d_scenario_id, &d_scenario_name, &d_scenario_description, &d_scenario_version, &d_score, &d_error_code, &dr_name, &dr_description, &dr_score_modifier, &dr_result, &dr_error_code}, func() error {

		// On first iteration, fill the decision object
		// Skip on future iterations
		if i == 0 {

			d.ID = d_id
			d.Created_at = d_created_at
			d.Outcome = scenarios.OutcomeFrom(d_outcome)
			d.ScenarioID = d_scenario_id
			d.ScenarioName = d_scenario_name
			d.ScenarioDescription = d_scenario_description
			d.ScenarioVersion = d_scenario_version
			d.Score = d_score
			d.RuleExecutions = make([]scenarios.RuleExecution, 0)
			d.DecisionError = scenarios.DecisionError(d_error_code)
		}

		d.RuleExecutions = append(d.RuleExecutions, scenarios.RuleExecution{
			Rule: scenarios.Rule{
				Name:        dr_name,
				Description: dr_description,
			},
			Result:              dr_result,
			ResultScoreModifier: dr_score_modifier,
			Error:               scenarios.RuleExecutionError(dr_error_code),
		})

		i++

		return nil
	})

	if i == 0 {
		return scenarios.Decision{}, app.ErrNotFoundInRepository
	}

	if err != nil {
		return scenarios.Decision{}, fmt.Errorf("error getting decision : %w", err)
	}

	return d, nil
}
