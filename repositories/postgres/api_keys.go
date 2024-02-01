package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (db *Database) CreateApiKey(ctx context.Context, apiKey models.CreateApiKeyInput) error {
	query := `
		INSERT INTO apikeys (org_id, key, description)
		VALUES ($1, $2, $3)
	`

	_, err := db.pool.Exec(ctx, query, apiKey.OrganizationId, apiKey.Key, apiKey.Description)
	if err != nil {
		return fmt.Errorf("pool.Exec error: %w", err)
	}
	return nil
}

func (db *Database) GetApiKeyByKey(ctx context.Context, key string) (models.ApiKey, error) {
	query := `
		SELECT id, org_id, key, description, role
		FROM apikeys
		WHERE key = $1
		AND deleted_at IS NULL
	`

	var apiKey dbmodels.DBApiKey
	err := db.pool.QueryRow(ctx, query, key).Scan(&apiKey.Id, &apiKey.OrganizationId, &apiKey.Key, &apiKey.Description, &apiKey.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ApiKey{}, models.NotFoundError
	}
	if err != nil {
		return models.ApiKey{}, fmt.Errorf("pool.QueryRow error: %w", err)
	}
	return dbmodels.AdaptApikey(apiKey)
}

func (db *Database) GetApiKeysOfOrganization(ctx context.Context, organizationID string) ([]models.ApiKey, error) {
	query := `
		SELECT id, org_id, key, description, role
		FROM apikeys
		WHERE org_id = $1
		AND deleted_at IS NULL
	`

	rows, err := db.pool.Query(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("pool.QueryRow error: %w", err)
	}

	apiKeys, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.ApiKey, error) {
		var apiKey dbmodels.DBApiKey
		if err := row.Scan(&apiKey.Id, &apiKey.OrganizationId, &apiKey.Key, &apiKey.Description, &apiKey.Role); err != nil {
			return models.ApiKey{}, fmt.Errorf("row.Scan error: %w", err)
		}
		return dbmodels.AdaptApikey(apiKey)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.NotFoundError
	}
	if err != nil {
		return nil, fmt.Errorf("pgx.CollectRows error: %w", err)
	}
	return apiKeys, nil
}
