package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type dbScenarioPublication struct {
	ID    string `db:"id"`
	OrgID string `db:"org_id"`
	// UserID              string    `db:"user_id"`
	ScenarioID          string    `db:"scenario_id"`
	ScenarioIterationID string    `db:"scenario_iteration_id"`
	PublicationAction   string    `db:"publication_action"`
	CreatedAt           time.Time `db:"created_at"`
}

func (sp *dbScenarioPublication) dto() app.ScenarioPublication {
	return app.ScenarioPublication{
		ID:    sp.ID,
		OrgID: sp.OrgID,
		// UserID:              sp.UserID,
		ScenarioID:          sp.ScenarioID,
		ScenarioIterationID: sp.ScenarioIterationID,
		PublicationAction:   app.PublicationActionFrom(sp.PublicationAction),
		CreatedAt:           sp.CreatedAt,
	}
}

func (r *PGRepository) ReadScenarioPublications(ctx context.Context, orgID string, scenarioID string) ([]app.ScenarioPublication, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("scenario_publications").
		Where(squirrel.Eq{
			"org_id":      orgID,
			"scenario_id": scenarioID,
		}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build scenario publication query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenarioPublications, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbScenarioPublication])

	scenarioPubblicationDTOs := make([]app.ScenarioPublication, len(scenarioPublications))
	for i, scenario := range scenarioPublications {
		scenarioPubblicationDTOs[i] = scenario.dto()
	}
	return scenarioPubblicationDTOs, err
}

func (r *PGRepository) ReadScenarioIterationPublications(ctx context.Context, orgID string, scenarioIterationID string) ([]app.ScenarioPublication, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("scenario_publications").
		Where(squirrel.Eq{
			"org_id":                orgID,
			"scenario_iteration_id": scenarioIterationID,
		}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build scenario publication query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenarioPublications, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbScenarioPublication])

	scenarioPubblicationDTOs := make([]app.ScenarioPublication, len(scenarioPublications))
	for i, scenario := range scenarioPublications {
		scenarioPubblicationDTOs[i] = scenario.dto()
	}
	return scenarioPubblicationDTOs, err
}

func (r *PGRepository) ReadScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (app.ScenarioPublication, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("scenario_publications").
		Where(squirrel.Eq{
			"org_id": orgID,
			"id":     scenarioPublicationID,
		}).ToSql()

	if err != nil {
		return app.ScenarioPublication{}, fmt.Errorf("unable to build scenario publication query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenarioPublication, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioPublication])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ScenarioPublication{}, app.ErrNotFoundInRepository
	} else if err != nil {
		return app.ScenarioPublication{}, fmt.Errorf("unable to get scenario publication: %w", err)
	}

	return scenarioPublication.dto(), err
}

type dbCreateScenarioPublication struct {
	OrgID string `db:"org_id"`
	// UserID              string `db:"user_id"`
	ScenarioID          string `db:"scenario_id"`
	ScenarioIterationID string `db:"scenario_iteration_id"`
	PublicationAction   string `db:"publication_action"`
}

func (r *PGRepository) createScenarioPublication(ctx context.Context, tx pgx.Tx, sp dbCreateScenarioPublication) (dbScenarioPublication, error) {
	sql, args, err := r.queryBuilder.
		Insert("scenario_publications").
		SetMap(upsertMapByName(sp)).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return dbScenarioPublication{}, fmt.Errorf("unable to build scenario publication query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	createdScenarioPublication, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioPublication])
	if err != nil {
		return dbScenarioPublication{}, fmt.Errorf("unable to create scenario publication: %w", err)
	}

	return createdScenarioPublication, err
}

func (r *PGRepository) CreateScenarioPublication(ctx context.Context, orgID string, sp app.CreateScenarioPublication) ([]app.ScenarioPublication, error) {
	scenario, err := r.GetScenario(ctx, orgID, sp.ScenarioID)
	if err != nil {
		return nil, app.ErrNotFoundInRepository
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call even if tx commits

	var scenarioPublications []app.ScenarioPublication
	switch sp.PublicationAction {
	case app.Publish:
		if scenario.LiveVersion != nil {
			unpublishOldIteration, err := r.createScenarioPublication(ctx, tx, dbCreateScenarioPublication{
				OrgID: orgID,
				// UserID: sp.UserID,
				ScenarioID:          sp.ScenarioID,
				ScenarioIterationID: scenario.LiveVersion.ID,
				PublicationAction:   app.Unpublish.String(),
			})
			if err != nil {
				return nil, fmt.Errorf("unable to unpublish old scenario iteration: %w", err)
			}
			scenarioPublications = append(scenarioPublications, unpublishOldIteration.dto())
		}

		publishNewIteration, err := r.createScenarioPublication(ctx, tx, dbCreateScenarioPublication{
			OrgID: orgID,
			// UserID: sp.UserID,
			ScenarioID:          sp.ScenarioID,
			ScenarioIterationID: sp.ScenarioIterationID,
			PublicationAction:   app.Publish.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("unable to publish new scenario iteration: %w", err)
		}
		scenarioPublications = append(scenarioPublications, publishNewIteration.dto())

		err = r.publishScenarioIteration(ctx, tx, orgID, sp.ScenarioIterationID)
		if err != nil {
			return nil, fmt.Errorf("unable to publish live scenario iteration(id: %s): %w", sp.ScenarioIterationID, err)
		}
	case app.Unpublish:
		if scenario.LiveVersion == nil || scenario.LiveVersion.ID != sp.ScenarioIterationID {
			return nil, fmt.Errorf("unable to unpublish scenario iteration(id: %s): current live scenario iteration point to a different scenario iteration(id: %s): %w", sp.ScenarioIterationID, scenario.LiveVersion.ID, err)
		}
		unpublishOldIteration, err := r.createScenarioPublication(ctx, tx, dbCreateScenarioPublication{
			OrgID: orgID,
			// UserID: sp.UserID,
			ScenarioID:          sp.ScenarioID,
			ScenarioIterationID: sp.ScenarioIterationID,
			PublicationAction:   app.Unpublish.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("unable to unpublish provided scenario iteration: %w", err)
		}
		scenarioPublications = append(scenarioPublications, unpublishOldIteration.dto())

		err = r.unpublishScenarioIteration(ctx, tx, orgID, sp.ScenarioID)
		if err != nil {
			return nil, fmt.Errorf("unable to unpublish scenario(id: %s): %w", sp.ScenarioID, err)
		}
	}

	tx.Commit(ctx)

	return scenarioPublications, err
}
