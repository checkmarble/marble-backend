package pg_repository

import (
	"context"
	"errors"
	"marble/marble-backend/app"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type dbScenario struct {
	ID                string    `db:"id"`
	OrgID             string    `db:"org_id"`
	Name              string    `db:"name"`
	Description       string    `db:"description"`
	TriggerObjectType string    `db:"trigger_object_type"`
	CreatedAt         time.Time `db:"created_at"`
	// LiveVersion       *ScenarioIteration `db:"-"`
}

func (s *dbScenario) dto() app.Scenario {
	return app.Scenario{
		ID:                s.ID,
		Name:              s.Name,
		Description:       s.Description,
		TriggerObjectType: s.TriggerObjectType,
		CreatedAt:         s.CreatedAt,
		// LiveVersion:       s.LiveVersion,
	}
}

func (r *PGRepository) GetScenarios(orgID string) ([]app.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("scenarios").
		Where(squirrel.Eq{"org_id": orgID}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, _ := r.db.Query(context.Background(), sql, args...)
	scenarios, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbScenario])

	scenarioDTOs := make([]app.Scenario, len(scenarios))
	for i, scenario := range scenarios {
		scenarioDTOs[i] = scenario.dto()
	}
	return scenarioDTOs, err
}

func (r *PGRepository) GetScenario(orgID string, scenarioID string) (app.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("scenarios").
		Where(squirrel.Eq{"org_id": orgID, "id": scenarioID}).ToSql()
	if err != nil {
		return app.Scenario{}, err
	}

	rows, _ := r.db.Query(context.Background(), sql, args...)
	scenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Scenario{}, app.ErrNotFoundInRepository
	}

	return scenario.dto(), err
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
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.Scenario{}, err
	}

	rows, _ := r.db.Query(context.Background(), sql, args...)
	createdScenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])

	return createdScenario.dto(), err
}
