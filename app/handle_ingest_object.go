package app

import (
	"context"
	"marble/marble-backend/models"

	"golang.org/x/exp/slog"
)

func (app *App) IngestObject(ctx context.Context, payload Payload, table models.Table, logger *slog.Logger) (err error) {
	return app.repository.IngestObject(ctx, payload, table, logger)
}
