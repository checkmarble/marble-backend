package pg_repository

import (
	"context"
	"marble/marble-backend/app"
)

func (r *PGRepository) GetScenarioIteration(orgID string, scenarioIterationID string) (scenarioIteration app.ScenarioIteration, err error) {

	// Scenario Iteration
	sql, args, err := r.queryBuilder.
		Select("si.id, si.name, si.description, si.version").
		From("scenario_iteration si").
		Join("scenario_iteration_ sir ON sir.scenario_iteration_id = si.id").
		ToSql()

	if err != nil {
		return app.ScenarioIteration{}, err
	}

	rows, err := r.db.Query(context.TODO(), sql, args)

	si := app.ScenarioIteration
	sib := app.ScenarioIterationBody

	var si_id string
	var si_name string
	var si_description string
	var si_version int

	var sib_trigger_condition []bytes


	// Loop counter
	i := 0

	_, err = pgx.ForEachRow(rows, []any{&si_id, &si_name, &si_description, &si_version }, func() error {
		
		if i == 0 {

			si.ID = si_id
			si.Name = si_name
			si.Description = si_description
			si.Version = si_version

			sib.TriggerCondition = JSON.Unmarshall

		}



	}

}
