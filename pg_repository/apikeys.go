package pg_repository

import (
	"context"
	"fmt"
)

const TABLE_APIKEYS = "apikeys"

type CreateApiKeyInput struct {
	OrgID string
	Key   string
}

func (r *PGRepository) CreateApiKey(ctx context.Context, apiKey CreateApiKeyInput) error {
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
		ToSql()
	if err != nil {
		return fmt.Errorf("unable to build apiKey query: %w", err)
	}

	_, err = r.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("unable to create apiKey: %w", err)
	}

	return nil
}
