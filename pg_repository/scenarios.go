package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype/zeronull"
)

type dbScenario struct {
	ID                string        `db:"id"`
	OrgID             string        `db:"org_id"`
	Name              string        `db:"name"`
	Description       string        `db:"description"`
	TriggerObjectType string        `db:"trigger_object_type"`
	CreatedAt         time.Time     `db:"created_at"`
	LiveVersionID     zeronull.Text `db:"live_scenario_iteration_id"`
}

func (s *dbScenario) dto() app.Scenario {
	return app.Scenario{
		ID:                s.ID,
		Name:              s.Name,
		Description:       s.Description,
		TriggerObjectType: s.TriggerObjectType,
		CreatedAt:         s.CreatedAt,
	}
}

func (r *PGRepository) GetScenarios(orgID string) ([]app.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("scenarios").
		Where(squirrel.Eq{
			"org_id": orgID,
		}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build scenario query: %w", err)
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
		Where(squirrel.Eq{
			"org_id": orgID,
			"id":     scenarioID,
		}).ToSql()

	if err != nil {
		return app.Scenario{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(context.Background(), sql, args...)
	scenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Scenario{}, app.ErrNotFoundInRepository
	} else if err != nil {
		return app.Scenario{}, fmt.Errorf("unable to get scenario: %w", err)
	}

	scenarioDTO := scenario.dto()

	if scenario.LiveVersionID == "" {
		return scenarioDTO, err
	}

	// liveScenarioIteration, err := r.GetScenarioIteration(orgID, scenario.LiveVersionID)

	// if errors.Is(err, pgx.ErrNoRows) {

	// 	// Silently ignore error, scenario will not point to a live version
	// 	// TODO: check how this is possible ?

	// 	return app.Scenario{}, err

	// } else if err != nil {

	// 	// Silently ignore error, scenario will not point to a live version
	// 	// TODO: check how this is possible ?

	// 	return app.Scenario{}, err
	// }

	// s.LiveVersion = &liveScenarioIteration

	return scenarioDTO, err
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
		return app.Scenario{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(context.Background(), sql, args...)
	createdScenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])
	if err != nil {
		return app.Scenario{}, fmt.Errorf("unable to create scenario: %w", err)
	}

	return createdScenario.dto(), err
}
