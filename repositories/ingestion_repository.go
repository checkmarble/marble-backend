package repositories

import (
	"context"
	"marble/marble-backend/models"

	"golang.org/x/exp/slog"
)

type IngestionRepository interface {
	IngestObject(ctx context.Context, payload models.Payload, table models.Table, logger *slog.Logger) (err error)
}
