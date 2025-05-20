package usecases

import (
	"context"
	"io"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
)

type offloadableRepository interface {
	GetOffloadedDecisionRuleKey(orgId, decisionId, ruleId string, createdAt time.Time) string
	GetOffloadingWatermark(ctx context.Context, exec repositories.Executor, orgId, table string) (*models.OffloadingWatermark, error)
}

type OffloadedReader struct {
	executorFactory     executor_factory.ExecutorFactory
	repository          offloadableRepository
	blobRepository      repositories.BlobRepository
	offloadingBucketUrl string
}

func (uc OffloadedReader) MutateWithOffloadedDecisionRules(ctx context.Context, orgId string,
	decision models.DecisionWithRuleExecutions,
) error {
	offloadingWatermark, err := uc.repository.GetOffloadingWatermark(ctx,
		uc.executorFactory.NewExecutor(), orgId, "decision_rules")
	if err != nil {
		return err
	}

	if offloadingWatermark == nil {
		return nil
	}
	if decision.CreatedAt.After(offloadingWatermark.WatermarkTime) {
		return nil
	}

	for idx, rule := range decision.RuleExecutions {
		key := uc.repository.GetOffloadedDecisionRuleKey(orgId, rule.DecisionId, rule.Rule.Id, decision.CreatedAt)

		blob, err := uc.blobRepository.GetBlob(ctx, uc.offloadingBucketUrl, key)
		if err != nil {
			// A missing rule before the watermark means it was null and can be skipped.
			if errors.Is(err, models.NotFoundError) {
				continue
			}

			return err
		}
		defer blob.ReadCloser.Close()

		content, err := io.ReadAll(blob.ReadCloser)
		if err != nil {
			return err
		}

		ruleEvaluation, err := dbmodels.DeserializeNodeEvaluationDto(content)
		if err != nil {
			return err
		}

		decision.RuleExecutions[idx].Evaluation = ruleEvaluation
	}

	return nil
}
