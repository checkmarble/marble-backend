package pg_repository

import (
	"context"
	"marble/marble-backend/app"
)

func (r *PGRepository) GetScenario(orgID string, scenarioID string) (app.Scenario, error) {
	queryString := `
	SELECT
	s.id,
	s.name,
	s.description,
	s.trigger_object_type,
	s.created_at
	
	FROM scenarios s
	WHERE s.org_id = $1
	AND s.id = $2
	`
	var s app.Scenario
	err := r.db.QueryRow(context.Background(), queryString, orgID, scenarioID).Scan(
		&s.ID,
		&s.Name,
		&s.Description,
		&s.TriggerObjectType,
		&s.CreatedAt,
	)
	return s, err
}

func (r *PGRepository) PostScenario(orgID string, scenario app.Scenario) (string, error) {
	// Use tx to prepare a possible Post of LiveVersion
	tx, err := r.db.Begin(context.Background())
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())

	insertDecisionQueryString := `
	INSERT INTO scenarios (
		"org_id",
		"id",
		"name",
		"description",
		"trigger_object_type",
		"created_at"
	  )
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING "id";
	`

	var createdDecisionID string
	err = tx.QueryRow(context.TODO(), insertDecisionQueryString,
		orgID,
		scenario.ID,
		scenario.Name,
		scenario.Description,
		scenario.TriggerObjectType,
		scenario.CreatedAt.UTC(),
	).Scan(&createdDecisionID)

	if err != nil {
		return "", err
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return "", err
	}

	return createdDecisionID, nil
}
