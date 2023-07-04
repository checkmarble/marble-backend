package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"
	"marble/marble-backend/utils"

	"github.com/jackc/pgx/v5"
)

type dbCreateScenario struct {
	Id                string `db:"id"`
	OrgID             string `db:"org_id"`
	Name              string `db:"name"`
	Description       string `db:"description"`
	TriggerObjectType string `db:"trigger_object_type"`
}

func (r *PGRepository) CreateScenario(ctx context.Context, orgID string, scenario models.CreateScenarioInput) (models.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Insert("scenarios").
		SetMap(ColumnValueMap(dbCreateScenario{
			Id:                utils.NewPrimaryKey(orgID),
			OrgID:             orgID,
			Name:              scenario.Name,
			Description:       scenario.Description,
			TriggerObjectType: scenario.TriggerObjectType,
		})).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return models.Scenario{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	createdScenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbmodels.DBScenario])
	if err != nil {
		return models.Scenario{}, fmt.Errorf("unable to create scenario: %w", err)
	}

	return dbmodels.AdaptScenario(createdScenario), nil
}

type dbUpdateScenarioInput struct {
	Name        *string `db:"name"`
	Description *string `db:"description"`
}

func (r *PGRepository) UpdateScenario(ctx context.Context, orgID string, scenario models.UpdateScenarioInput) (models.Scenario, error) {
	sql, args, err := r.queryBuilder.
		Update("scenarios").
		SetMap(ColumnValueMap(dbUpdateScenarioInput{
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
	updatedScenario, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbmodels.DBScenario])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Scenario{}, models.NotFoundInRepositoryError
	} else if err != nil {
		return models.Scenario{}, fmt.Errorf("unable to update scenario(id: %s): %w", scenario.ID, err)
	}

	return dbmodels.AdaptScenario(updatedScenario), nil
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
