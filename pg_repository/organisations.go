package pg_repository

import (
	"context"
	"errors"
	"fmt"

	"marble/marble-backend/app"

	"github.com/jackc/pgx/v5"
)

type dbOrganization struct {
	ID           string `db:"id"`
	Name         string `db:"name"`
	DatabaseName string `db:"database_name"`
}

func (org *dbOrganization) dto() app.Organization {
	return app.Organization{
		ID:   org.ID,
		Name: org.Name,
	}
}

func (r *PGRepository) GetOrganization(ctx context.Context, orgID string) (app.Organization, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("organizations o").
		Where("o.id = ?", orgID).
		ToSql()
	if err != nil {
		return app.Organization{}, fmt.Errorf("unable to build organization query: %w", err)
	}

	type DBRow struct {
		dbOrganization
		tokens []dbToken
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	organization, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[DBRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Organization{}, app.ErrNotFoundInRepository
	} else if err != nil {
		return app.Organization{}, fmt.Errorf("unable to get organization(id: %s): %w", orgID, err)
	}

	organizationDTO := organization.dto()
	return organizationDTO, nil
}

func (r *PGRepository) GetOrganizations(ctx context.Context) ([]app.Organization, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("organizations").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build organization query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	organizations, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbOrganization])
	if err != nil {
		return nil, fmt.Errorf("unable to get organizations: %w", err)
	}

	organizationDTOs := make([]app.Organization, len(organizations))
	for i, org := range organizations {
		organizationDTOs[i] = org.dto()
	}
	return organizationDTOs, nil
}

func (r *PGRepository) CreateOrganization(ctx context.Context, organisation app.CreateOrganizationInput) (app.Organization, error) {
	sql, args, err := r.queryBuilder.
		Insert("organizations").
		Columns(
			"name",
			"database_name",
		).
		Values(
			organisation.Name,
			organisation.DatabaseName,
		).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.Organization{}, fmt.Errorf("unable to build organization query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	createdOrg, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbOrganization])
	if err != nil {
		return app.Organization{}, fmt.Errorf("unable to create organization: %w", err)
	}

	return createdOrg.dto(), nil
}
