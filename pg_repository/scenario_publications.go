package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type dbScenarioPublication struct {
	ID    string `db:"id"`
	Rank  int32  `db:"rank"`
	OrgID string `db:"org_id"`
	// UserID              string    `db:"user_id"`
	ScenarioID          string    `db:"scenario_id"`
	ScenarioIterationID string    `db:"scenario_iteration_id"`
	PublicationAction   string    `db:"publication_action"`
	CreatedAt           time.Time `db:"created_at"`
}

func (sp *dbScenarioPublication) toDomain() app.ScenarioPublication {
	return app.ScenarioPublication{
		ID:    sp.ID,
		Rank:  sp.Rank,
		OrgID: sp.OrgID,
		// UserID:              sp.UserID,
		ScenarioID:          sp.ScenarioID,
		ScenarioIterationID: sp.ScenarioIterationID,
		PublicationAction:   app.PublicationActionFrom(sp.PublicationAction),
		CreatedAt:           sp.CreatedAt,
	}
}

type ListScenarioPublicationsFilters struct {
	ScenarioID *string `db:"scenario_id"`
	// UserID              *string    `db:"user_id"`
	ScenarioIterationID *string `db:"scenario_iteration_id"`
	PublicationAction   *string `db:"publication_action"`
}

func (r *PGRepository) ListScenarioPublications(ctx context.Context, orgID string, filters app.ListScenarioPublicationsFilters) ([]app.ScenarioPublication, error) {
	sql, args, err := r.queryBuilder.
		Select(columnList[dbScenarioPublication]()...).
		From("scenario_publications").
		Where("org_id = ?", orgID).
		Where(sq.Eq(columnValueMap(ListScenarioPublicationsFilters{
			ScenarioID: filters.ScenarioID,
			// UserID:              filters.UserID,
			ScenarioIterationID: filters.ScenarioIterationID,
			PublicationAction:   filters.PublicationAction,
		}))).
		OrderBy("rank DESC").ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build scenario publication query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	scenarioPublications, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbScenarioPublication])

	scenarioPubblicationDTOs := make([]app.ScenarioPublication, len(scenarioPublications))
	for i, scenario := range scenarioPublications {
		scenarioPubblicationDTOs[i] = scenario.toDomain()
	}
	return scenarioPubblicationDTOs, err
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
		SetMap(columnValueMap(sp)).
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

func (r *PGRepository) CreateScenarioPublication(ctx context.Context, orgID string, sp app.CreateScenarioPublicationInput) ([]app.ScenarioPublication, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call even if tx commits

	sql, args, err := r.queryBuilder.
		Select("s.id, s.live_scenario_iteration_id").
		From("scenario_iterations si").
		Join("scenarios s on s.id = si.scenario_id").
		Where("si.id = ?", sp.ScenarioIterationID).
		Where("si.org_id = ?", orgID).ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build scenario publication query: %w", err)
	}

	var scenarioID string
	var liveSIID *string
	err = tx.QueryRow(ctx, sql, args...).Scan(&scenarioID, &liveSIID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, app.ErrNotFoundInRepository
	} else if err != nil {
		return nil, fmt.Errorf("unable to query scenario iteration: %w", err)
	}

	var scenarioPublications []app.ScenarioPublication
	switch sp.PublicationAction {
	case app.Publish:
		if liveSIID != nil {
			if *liveSIID == sp.ScenarioIterationID {
				return nil, fmt.Errorf("scenario iteration(id: %s) is already live", *liveSIID)
			}
			unpublishOldIteration, err := r.createScenarioPublication(ctx, tx, dbCreateScenarioPublication{
				OrgID: orgID,
				// UserID: sp.UserID,
				ScenarioID:          scenarioID,
				ScenarioIterationID: *liveSIID,
				PublicationAction:   app.Unpublish.String(),
			})
			if err != nil {
				return nil, fmt.Errorf("unable to unpublish old scenario iteration: %w", err)
			}
			scenarioPublications = append(scenarioPublications, unpublishOldIteration.toDomain())
		}

		err = r.publishScenarioIteration(ctx, tx, orgID, sp.ScenarioIterationID)
		if err != nil && !errors.Is(err, ErrAlreadyPublished) {
			return nil, fmt.Errorf("unable to publish scenario iteration: \n%w", err)
		}

		publishNewIteration, err := r.createScenarioPublication(ctx, tx, dbCreateScenarioPublication{
			OrgID: orgID,
			// UserID: sp.UserID,
			ScenarioID:          scenarioID,
			ScenarioIterationID: sp.ScenarioIterationID,
			PublicationAction:   app.Publish.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("unable to publish new scenario iteration: \n%w", err)
		}
		scenarioPublications = append(scenarioPublications, publishNewIteration.toDomain())

		err = r.setLiveScenarioIteration(ctx, tx, orgID, sp.ScenarioIterationID)
		if err != nil {
			return nil, fmt.Errorf("unable to publish live scenario iteration(id: %s): \n%w", sp.ScenarioIterationID, err)
		}

	case app.Unpublish:
		if liveSIID == nil || *liveSIID != sp.ScenarioIterationID {
			return nil, fmt.Errorf("unable to unpublish: scenario iteration(id: %s) is not live", sp.ScenarioIterationID)
		}
		unpublishOldIteration, err := r.createScenarioPublication(ctx, tx, dbCreateScenarioPublication{
			OrgID: orgID,
			// UserID: sp.UserID,
			ScenarioID:          scenarioID,
			ScenarioIterationID: sp.ScenarioIterationID,
			PublicationAction:   app.Unpublish.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("unable to unpublish provided scenario iteration: \n%w", err)
		}
		scenarioPublications = append(scenarioPublications, unpublishOldIteration.toDomain())

		err = r.unsetLiveScenarioIteration(ctx, tx, orgID, scenarioID)
		if err != nil {
			return nil, fmt.Errorf("unable to unpublish scenario(id: %s): \n%w", scenarioID, err)
		}

	default:
		return nil, fmt.Errorf("unknown publication action")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("transaction issue: \n%w", err)
	}

	return scenarioPublications, nil
}

func (r *PGRepository) GetScenarioPublication(ctx context.Context, orgID string, ID string) (app.ScenarioPublication, error) {
	sql, args, err := r.queryBuilder.
		Select(columnList[dbScenarioPublication]()...).
		From("scenario_publications").
		Where("org_id = ?", orgID).
		Where("id = ?", ID).
		OrderBy("rank DESC").ToSql()
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

	return scenarioPublication.toDomain(), err
}
