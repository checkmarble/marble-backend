package pg_repository

import (
	"context"

	"marble/marble-backend/app"
)

func (r *PGRepository) StoreDecision(orgID string, decision app.Decision) (id string, err error) {

	// Begin a transaction to store decision + rules in 1 go

	tx, err := r.db.Begin(context.Background())
	if err != nil {
		return "", err
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	defer tx.Rollback(context.Background())

	// insert the decision
	insertDecisionQueryString := `
	INSERT INTO decisions
	("org_id",
	"created_at",
	"outcome",
	"scenario_id",
	"scenario_name",
	"scenario_description",
	"scenario_version",
	"score",
	"error_code")

	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	
	RETURNING "id";
	`

	var createdDecisionID string
	err = tx.QueryRow(context.TODO(), insertDecisionQueryString,
		orgID,
		decision.Created_at.UTC(),
		decision.Outcome.String(),
		decision.ScenarioID,
		decision.ScenarioName,
		decision.ScenarioDescription,
		decision.ScenarioVersion,
		decision.Score,
		decision.DecisionError,
	).Scan(&createdDecisionID)

	if err != nil {
		return "", err
	}

	// insert rules
	insertRuleQueryString := `
	INSERT INTO decision_rules
		("org_id",
    	"decision_id",
    	"name",
    	"description",
   		"score_modifier",
    	"result",
		"error_code")

	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	// Loop over each rule execution, add it to the transaction
	for _, re := range decision.RuleExecutions {

		_, err = tx.Exec(context.Background(), insertRuleQueryString, orgID, createdDecisionID, re.Rule.Name, re.Rule.Description, re.ResultScoreModifier, re.Result, re.Error)
		if err != nil {
			return "", err
		}

	}

	err = tx.Commit(context.Background())
	if err != nil {
		return "", err
	}

	return createdDecisionID, nil
}
