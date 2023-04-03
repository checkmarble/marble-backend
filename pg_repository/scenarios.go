package pg_repository

import (
	"context"
	"marble/marble-backend/app"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

func (r *PGRepository) GetScenarios(orgID string) ([]app.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Select("id", "name", "description", "trigger_object_type", "created_at").
		From("scenarios").
		Where(squirrel.Eq{"org_id": orgID}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(context.Background(), sql, args...)
	if err != nil {
		return nil, err
	}

	scenarios, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (app.Scenario, error) {
		s := app.Scenario{}
		err := row.Scan(&s.ID, &s.Name, &s.Description, &s.TriggerObjectType, &s.CreatedAt)
		return s, err
	})

	return scenarios, err
}

func (r *PGRepository) GetScenario(orgID string, scenarioID string) (s app.Scenario, err error) {
	sql, args, err := r.queryBuilder.
		Select("id", "name", "description", "trigger_object_type", "created_at").
		From("scenarios").
		Where(squirrel.Eq{"org_id": orgID, "id": scenarioID}).ToSql()
	if err != nil {
		return
	}

	err = r.db.QueryRow(context.Background(), sql, args...).Scan(
		&s.ID,
		&s.Name,
		&s.Description,
		&s.TriggerObjectType,
		&s.CreatedAt,
	)
	return
}

func (r *PGRepository) PostScenario(orgID string, scenario app.Scenario) (app.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Insert("scenarios").
		Columns(
			"org_id",
			"name",
			"description",
			"trigger_object_type",
		).
		Values(
			orgID,
			scenario.Name,
			scenario.Description,
			scenario.TriggerObjectType,
		).
		Suffix("RETURNING \"id\", \"created_at\"").ToSql()
	if err != nil {
		return scenario, err
	}

	err = r.db.QueryRow(context.TODO(), sql, args...).Scan(&scenario.ID, &scenario.CreatedAt)

	return scenario, err
}
