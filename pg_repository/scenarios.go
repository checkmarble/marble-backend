package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"sort"
	"sync"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type dbScenario struct {
	ID                string      `db:"id"`
	OrgID             string      `db:"org_id"`
	Name              string      `db:"name"`
	Description       string      `db:"description"`
	TriggerObjectType string      `db:"trigger_object_type"`
	CreatedAt         time.Time   `db:"created_at"`
	LiveVersionID     pgtype.Text `db:"live_scenario_iteration_id"`
	DeletedAt         pgtype.Time `db:"deleted_at"`
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

func (r *PGRepository) addLiveVersionToScenario(ctx context.Context, orgID string, scenario dbScenario) (app.Scenario, error) {
	scenarioDTO := scenario.dto()

	if scenario.LiveVersionID.Valid {
		liveScenarioIteration, err := r.GetScenarioIteration(ctx, orgID, scenario.LiveVersionID.String)
		if err != nil {
			return app.Scenario{}, fmt.Errorf("unable to get live scenario iteration: %w", err)
		}
		liveVersion, err := app.NewPublishedScenarioIteration(liveScenarioIteration)
		if err != nil {
			return app.Scenario{}, app.ErrScenarioIterationNotValid
		}
		scenarioDTO.LiveVersion = &liveVersion
	}

	return scenarioDTO, nil
}

func (r *PGRepository) ListScenarios(ctx context.Context, orgID string) ([]app.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Select(columnList[dbScenario]()...).
		From("scenarios").
		Where(squirrel.Eq{
			"org_id": orgID,
		}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenarios, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbScenario])
	if err != nil {
		return nil, fmt.Errorf("unable to get scenarios: %w", err)
	}

	// Getting live versions asynchronously, so we need to re-sort the scenarios after
	out := make(chan app.Scenario, len(scenarios))
	errs := make(chan error, len(scenarios))
	var wg sync.WaitGroup
	wg.Add(len(scenarios))
	for i, scenario := range scenarios {
		go func(i int, scenario dbScenario) {
			defer wg.Done()
			scenarioDTO, err := r.addLiveVersionToScenario(ctx, orgID, scenario)
			if err != nil {
				errs <- fmt.Errorf("unable to get live version for scenario: %w", err)
			}
			out <- scenarioDTO
		}(i, scenario)
	}
	wg.Wait()
	close(out)
	close(errs)

	scenarioDTOs := make([]app.Scenario, len(scenarios))
	if len(errs) > 0 {
		return scenarioDTOs, <-errs
	}
	for i := range scenarioDTOs {
		scenarioDTOs[i] = <-out
	}
	sort.Slice(scenarioDTOs, func(i, j int) bool {
		return scenarioDTOs[i].CreatedAt.Before(scenarioDTOs[j].CreatedAt)
	})
	return scenarioDTOs, nil
}

func (r *PGRepository) GetScenario(ctx context.Context, orgID string, scenarioID string) (app.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Select(columnList[dbScenario]()...).
		From("scenarios").
		Where(squirrel.Eq{
			"org_id": orgID,
			"id":     scenarioID,
		}).ToSql()

	if err != nil {
		return app.Scenario{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Scenario{}, app.ErrNotFoundInRepository
	} else if err != nil {
		return app.Scenario{}, fmt.Errorf("unable to get scenario: %w", err)
	}

	return r.addLiveVersionToScenario(ctx, orgID, scenario)
}

type dbCreateScenario struct {
	OrgID             string `db:"org_id"`
	Name              string `db:"name"`
	Description       string `db:"description"`
	TriggerObjectType string `db:"trigger_object_type"`
}

func (r *PGRepository) CreateScenario(ctx context.Context, orgID string, scenario app.CreateScenarioInput) (app.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Insert("scenarios").
		SetMap(columnValueMap(dbCreateScenario{
			OrgID:             orgID,
			Name:              scenario.Name,
			Description:       scenario.Description,
			TriggerObjectType: scenario.TriggerObjectType,
		})).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.Scenario{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	createdScenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])
	if err != nil {
		return app.Scenario{}, fmt.Errorf("unable to create scenario: %w", err)
	}

	return r.addLiveVersionToScenario(ctx, orgID, createdScenario)
}

type dbUpdateScenarioInput struct {
	Name        *string `db:"name"`
	Description *string `db:"description"`
}

func (r *PGRepository) UpdateScenario(ctx context.Context, orgID string, scenario app.UpdateScenarioInput) (app.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Update("scenarios").
		SetMap(columnValueMap(dbUpdateScenarioInput{
			Name:        scenario.Name,
			Description: scenario.Description,
		})).
		Where("id = ?", scenario.ID).
		Where("org_id = ?", orgID).
		Suffix("RETURNING *").
		ToSql()
	if err != nil {
		return app.Scenario{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	updatedScenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Scenario{}, app.ErrNotFoundInRepository
	} else if err != nil {
		return app.Scenario{}, fmt.Errorf("unable to update scenario(id: %s): %w", scenario.ID, err)
	}

	return r.addLiveVersionToScenario(ctx, orgID, updatedScenario)
}

func (r *PGRepository) setLiveScenarioIteration(ctx context.Context, tx pgx.Tx, orgID string, scenarioIterationID string) error {
	sql, args, err := r.queryBuilder.
		Update("scenarios").
		Set("live_scenario_iteration_id", scenarioIterationID).
		From("scenario_iterations si").
		Where("si.id = ?", scenarioIterationID).
		Where("si.org_id = ?", orgID).
		Where("scenarios.id = si.scenario_id").
		ToSql()

	if err != nil {
		return fmt.Errorf("unable to build query: %w", err)
	}

	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("unable to run query: %w", err)
	}

	return nil
}

func (r *PGRepository) unsetLiveScenarioIteration(ctx context.Context, tx pgx.Tx, orgID string, scenarioID string) error {
	sql, args, err := r.queryBuilder.
		Update("scenarios").
		Set("live_scenario_iteration_id", nil).
		Where("id = ?", scenarioID).
		Where("org_id = ?", orgID).
		ToSql()

	if err != nil {
		return fmt.Errorf("unable to build query: %w", err)
	}

	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("unable to run query: %w", err)
	}

	return nil
}
