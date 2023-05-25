package repositories

import (
	"context"
	"marble/marble-backend/app"
	"marble/marble-backend/models"

	"golang.org/x/exp/slog"
)

type RepositoryClientData interface {
	IngestObject(ctx context.Context, payload app.Payload, table models.Table, logger *slog.Logger) (err error)
	GetDbField(ctx context.Context, readParams app.DbFieldReadParams) (interface{}, error)
}
