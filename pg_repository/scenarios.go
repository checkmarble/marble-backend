package pg_repository

import (
	"context"
	"marble/marble-backend/app"

	"github.com/Masterminds/squirrel"
)

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

func (r *PGRepository) PostScenario(orgID string, scenario app.Scenario) (id string, err error) {
	sql, args, err := r.queryBuilder.
		Insert("scenarios").
		Columns(
			"org_id",
			"id",
			"name",
			"description",
			"trigger_object_type",
			"created_at").
		Values(
			orgID,
			scenario.ID,
			scenario.Name,
			scenario.Description,
			scenario.TriggerObjectType,
			scenario.CreatedAt.UTC()).
		Suffix("RETURNING \"id\"").ToSql()
	if err != nil {
		return
	}

	err = r.db.QueryRow(context.TODO(), sql, args).Scan(&id)

	return
}
