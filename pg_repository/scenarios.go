package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"time"

	sq "github.com/Masterminds/squirrel"
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
	ScenarioType      string      `db:"scenario_type"`
}

func (s *dbScenario) toDomain() models.Scenario {
	scenario := models.Scenario{
		ID:                s.ID,
		Name:              s.Name,
		Description:       s.Description,
		TriggerObjectType: s.TriggerObjectType,
		CreatedAt:         s.CreatedAt,
		ScenarioType:      models.ScenarioTypeFrom(s.ScenarioType),
	}
	if s.LiveVersionID.Valid {
		scenario.LiveVersionID = &s.LiveVersionID.String
	}
	return scenario
}

func (r *PGRepository) ListScenarios(ctx context.Context, orgID string, filters models.ListScenariosFilters) ([]models.Scenario, error) {
	query := r.queryBuilder.
		Select(ColumnList[dbScenario]()...).
		From("scenarios").
		Where(sq.Eq{
			"org_id": orgID,
		})
	if filters.IsActive != nil {
		if *filters.IsActive {
			query = query.Where("live_scenario_iteration_id IS NOT NULL")
		} else {
			query = query.Where("live_scenario_iteration_id IS NULL")
		}
	}
	if filters.ScenarioType != nil {
		query = query.Where(sq.Eq{
			"scenario_type": filters.ScenarioType.String(),
		})
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenarios, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbScenario])
	if err != nil {
		return nil, fmt.Errorf("unable to get scenarios: %w", err)
	}

	scenarioDTOs := []models.Scenario{}
	for _, s := range scenarios {
		scenarioDTOs = append(scenarioDTOs, s.toDomain())
	}
	return scenarioDTOs, nil
}

func (r *PGRepository) GetScenario(ctx context.Context, orgID string, scenarioID string) (models.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Select(ColumnList[dbScenario]()...).
		From("scenarios").
		Where(sq.Eq{
			"org_id": orgID,
			"id":     scenarioID,
		}).ToSql()

	if err != nil {
		return models.Scenario{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Scenario{}, models.NotFoundInRepositoryError
	} else if err != nil {
		return models.Scenario{}, fmt.Errorf("unable to get scenario: %w", err)
	}

	return scenario.toDomain(), nil
}

type dbCreateScenario struct {
	Id                string `db:"id"`
	OrgID             string `db:"org_id"`
	Name              string `db:"name"`
	Description       string `db:"description"`
	TriggerObjectType string `db:"trigger_object_type"`
	ScenarioType      string `db:"scenario_type"`
}

func (r *PGRepository) CreateScenario(ctx context.Context, orgID string, scenario models.CreateScenarioInput) (models.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Insert("scenarios").
		SetMap(columnValueMap(dbCreateScenario{
			Id:                utils.NewPrimaryKey(orgID),
			OrgID:             orgID,
			Name:              scenario.Name,
			Description:       scenario.Description,
			TriggerObjectType: scenario.TriggerObjectType,
			ScenarioType:      scenario.ScenarioType.String(),
		})).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return models.Scenario{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	createdScenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])
	if err != nil {
		return models.Scenario{}, fmt.Errorf("unable to create scenario: %w", err)
	}

	return createdScenario.toDomain(), nil
}

type dbUpdateScenarioInput struct {
	Name        *string `db:"name"`
	Description *string `db:"description"`
}

func (r *PGRepository) UpdateScenario(ctx context.Context, orgID string, scenario models.UpdateScenarioInput) (models.Scenario, error) {
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
		return models.Scenario{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	updatedScenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenario])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Scenario{}, models.NotFoundInRepositoryError
	} else if err != nil {
		return models.Scenario{}, fmt.Errorf("unable to update scenario(id: %s): %w", scenario.ID, err)
	}

	return updatedScenario.toDomain(), nil
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
