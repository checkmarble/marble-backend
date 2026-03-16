package ai_agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
)

type ScreeningHitSuggestionWorker struct {
	river.WorkerDefaults[models.ScreeningHitSuggestionArgs]
	aiAgentUsecase *AiAgentUsecase
	timeout        time.Duration
}

func NewScreeningHitSuggestionWorker(
	aiAgentUsecase *AiAgentUsecase,
	timeout time.Duration,
) ScreeningHitSuggestionWorker {
	return ScreeningHitSuggestionWorker{
		aiAgentUsecase: aiAgentUsecase,
		timeout:        timeout,
	}
}

func (w *ScreeningHitSuggestionWorker) Timeout(job *river.Job[models.ScreeningHitSuggestionArgs]) time.Duration {
	return w.timeout
}

func (w *ScreeningHitSuggestionWorker) Work(ctx context.Context, job *river.Job[models.ScreeningHitSuggestionArgs]) error {
	logger := utils.LoggerFromContext(ctx).With(slog.String("screening_id", job.Args.ScreeningId))
	ctx = utils.StoreLoggerInContext(ctx, logger)

	logger.InfoContext(ctx, "Starting screening hit suggestion job")

	err := w.aiAgentUsecase.AnalyseScreeningHits(ctx, job.Args.ScreeningId)
	if err != nil {
		logger.ErrorContext(ctx, "Screening hit suggestion job failed",
			"error", err,
		)
		return err
	}

	return nil
}
