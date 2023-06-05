package pg_repository

import (
	"context"
	"errors"
	"fmt"

	"marble/marble-backend/app"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBApiKey struct {
	ID        string      `db:"id"`
	OrgID     string      `db:"org_id"`
	Key       string      `db:"key"`
	DeletedAt pgtype.Time `db:"deleted_at"`
	Role      int         `db:"role"`
}

var ApiKeyFields = []string{"id", "org_id", "key", "deleted_at", "role"}

const TABLE_APIKEYS = "apikeys"

func (dto *DBApiKey) toDomain() models.ApiKey {
	return models.ApiKey{
		ApiKeyId:       models.ApiKeyId(dto.ID),
		OrganizationId: dto.OrgID,
		Key:            dto.Key,
		Role:           models.Role(dto.Role),
	}
}

func (r *PGRepository) GetOrganizationIDFromApiKey(ctx context.Context, key string) (orgID string, err error) {
	sql, args, err := r.queryBuilder.
		Select(ApiKeyFields...).
		From(TABLE_APIKEYS).
		Where("key = ?", key).
		ToSql()
	if err != nil {
		return "", fmt.Errorf("unable to build apiKey query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	apiKey, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[DBApiKey])
	if errors.Is(err, pgx.ErrNoRows) {
		return "", app.ErrNotFoundInRepository
	} else if err != nil {
		return "", fmt.Errorf("unable to get org from apiKey %s: %w", key, err)
	}

	return apiKey.OrgID, nil
}

func (r *PGRepository) GetApiKeyOfOrganization(ctx context.Context, orgID string) ([]models.ApiKey, error) {
	sql, args, err := r.queryBuilder.
		Select(ApiKeyFields...).
		From(TABLE_APIKEYS).
		Where("org_id = ?", orgID).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build apikey query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	apiKeys, err := pgx.CollectRows(rows, pgx.RowToStructByName[DBApiKey])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, app.ErrNotFoundInRepository
	} else if err != nil {
		return nil, fmt.Errorf("unable to get apiKeys for org(id: %s): %w", orgID, err)
	}

	return utils.Map(apiKeys, func(apikey DBApiKey) models.ApiKey {
		return apikey.toDomain()
	}), nil
}

type CreateApiKey struct {
	OrgID string
	Key   string
}

func (r *PGRepository) CreateApiKey(ctx context.Context, apiKey CreateApiKey) (models.ApiKey, error) {
	sql, args, err := r.queryBuilder.
		Insert(TABLE_APIKEYS).
		Columns(
			"org_id",
			"key",
		).
		Values(
			apiKey.OrgID,
			apiKey.Key,
		).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return models.ApiKey{}, fmt.Errorf("unable to build apikey query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	createdApiKey, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[DBApiKey])
	if err != nil {
		return models.ApiKey{}, fmt.Errorf("unable to create apikey: %w", err)
	}

	return createdApiKey.toDomain(), nil
}
