package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"

	"golang.org/x/exp/slog"
)

type IngestionUseCase struct {
	ingestionRepository repositories.IngestionRepository
}

func (usecase *IngestionUseCase) IngestObject(ctx context.Context, payload models.Payload, table models.Table, logger *slog.Logger) (err error) {
	return usecase.ingestionRepository.IngestObject(ctx, payload, table, logger)
}
