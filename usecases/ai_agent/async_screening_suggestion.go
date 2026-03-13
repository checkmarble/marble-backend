package ai_agent

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
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
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Starting screening hit suggestion job",
		"screening_id", job.Args.ScreeningId,
		"organization_id", job.Args.OrganizationId,
	)

	err := w.aiAgentUsecase.AnalyseScreeningHits(ctx, job.Args.ScreeningId, job.Args.OrganizationId)
	if err != nil {
		switch {
		case errors.Is(err, models.ForbiddenError):
			logger.WarnContext(ctx, "Skipping screening hit suggestion job due to insufficient permissions",
				"screening_id", job.Args.ScreeningId,
				"error", err,
			)
			return nil
		default:
			logger.ErrorContext(ctx, "Screening hit suggestion job failed",
				"screening_id", job.Args.ScreeningId,
				"error", err,
			)
			return err
		}
	}

	return nil
}
