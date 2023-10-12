package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
)

func (db *Database) CreateApiKey(ctx context.Context, apiKey models.CreateApiKeyInput) error {
	query := `
		INSERT INTO apikeys (org_id, key)
		VALUES ($1, $2)
	`

	_, err := db.pool.Exec(ctx, query, apiKey.OrganizationId, apiKey.Key)
	if err != nil {
		return fmt.Errorf("pool.Exec error: %w", err)
	}
	return nil
}

func (db *Database) GetApiKeyByKey(ctx context.Context, key string) (models.ApiKey, error) {
	query := `
		SELECT id, org_id, role
		FROM apikeys
		WHERE key = $1
	`

	var apiKey models.ApiKey
	err := db.pool.QueryRow(ctx, query, key).Scan(&apiKey.ApiKeyId, &apiKey.OrganizationId, &apiKey.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ApiKey{}, models.NotFoundError
	}
	if err != nil {
		return models.ApiKey{}, fmt.Errorf("pool.QueryRow error: %w", err)
	}
	return apiKey, nil
}

func (db *Database) GetApiKeysOfOrganization(ctx context.Context, organizationID string) ([]models.ApiKey, error) {
	query := `
		SELECT id, org_id, role
		FROM apikeys
		WHERE org_id = $1
	`

	rows, err := db.pool.Query(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("pool.QueryRow error: %w", err)
	}

	keys, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.ApiKey, error) {
		var key models.ApiKey
		if err := row.Scan(&key.ApiKeyId, &key.OrganizationId, &key.Role); err != nil {
			return models.ApiKey{}, fmt.Errorf("row.Scan error: %w", err)
		}
		return key, nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.NotFoundError
	}
	if err != nil {
		return nil, fmt.Errorf("pgx.CollectRows error: %w", err)
	}
	return keys, nil
}
