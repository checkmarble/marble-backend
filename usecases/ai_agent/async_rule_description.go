package ai_agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type RuleDescriptionUsecase interface {
	GenerateAndSaveRuleDescription(ctx context.Context, ruleId string) error
}

type RuleDescriptionWorker struct {
	river.WorkerDefaults[models.RuleDescriptionArgs]
	aiAgentUsecase RuleDescriptionUsecase
	timeout        time.Duration
}

func NewRuleDescriptionWorker(
	aiAgentUsecase RuleDescriptionUsecase,
	timeout time.Duration,
) RuleDescriptionWorker {
	return RuleDescriptionWorker{
		aiAgentUsecase: aiAgentUsecase,
		timeout:        timeout,
	}
}

func (w *RuleDescriptionWorker) Timeout(job *river.Job[models.RuleDescriptionArgs]) time.Duration {
	return w.timeout
}

func (w *RuleDescriptionWorker) Work(ctx context.Context, job *river.Job[models.RuleDescriptionArgs]) error {
	logger := utils.LoggerFromContext(ctx).With(slog.String("rule_id", job.Args.RuleId))
	ctx = utils.StoreLoggerInContext(ctx, logger)

	err := w.aiAgentUsecase.GenerateAndSaveRuleDescription(ctx, job.Args.RuleId)
	if err == nil {
		return nil
	}

	if errors.Is(err, models.LLMRateLimitedError) {
		logger.WarnContext(ctx, "LLM provider rate limited, rescheduling rule description job", "error", err.Error())
		return river.JobSnooze(30 * time.Second)
	}

	logger.ErrorContext(ctx, "Rule description generation failed, retrying", "error", err)
	return err
}
