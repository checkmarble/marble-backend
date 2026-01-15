package repositories

import (
	"context"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"gocloud.dev/blob"
)

type offloadableRepository interface {
	GetOffloadedDecisionRuleKey(orgId uuid.UUID, decisionId, ruleId, outcome string, createdAt time.Time) string
	GetOffloadedDecisionEvaluationKey(orgId uuid.UUID, decision models.Decision) string
	GetWatermark(
		ctx context.Context,
		exec Executor,
		orgId *uuid.UUID,
		watermarkType models.WatermarkType,
	) (*models.Watermark, error)
}

type OffloadedReadWriter struct {
	Repository          offloadableRepository
	BlobRepository      BlobRepository
	OffloadingBucketUrl string
}

func (uc OffloadedReadWriter) OffloadRuleExecutions(
	ctx context.Context,
	orgId uuid.UUID,
	decision models.Decision,
	evaluation []byte,
) error {
	if uc.OffloadingBucketUrl == "" {
		return nil
	}

	opts := blob.WriterOptions{}
	opts.BeforeWrite = func(asFunc func(any) bool) error {
		var gcsWriter *storage.Writer

		if asFunc(&gcsWriter) {
			gcsWriter.CustomTime = decision.CreatedAt
			gcsWriter.ChunkSize = 0
		}

		return nil
	}

	wr, err := uc.BlobRepository.OpenStreamWithOptions(ctx,
		uc.OffloadingBucketUrl,
		uc.Repository.GetOffloadedDecisionEvaluationKey(orgId, decision),
		&opts)
	if err != nil {
		return err
	}
	defer wr.Close()

	if _, err := wr.Write(evaluation); err != nil {
		return err
	}

	return nil
}

func (uc OffloadedReadWriter) MutateWithOffloadedDecisionRules(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	decision models.DecisionWithRuleExecutions,
) error {
	if uc.OffloadingBucketUrl == "" {
		return nil
	}

	bucket, err := uc.BlobRepository.RawBucket(ctx, uc.OffloadingBucketUrl)
	if err != nil {
		return err
	}

	decisionEvaluationKey := uc.Repository.GetOffloadedDecisionEvaluationKey(orgId, decision.Decision)

	exists, err := bucket.Exists(ctx, decisionEvaluationKey)
	if err != nil {
		return err
	}

	if exists {
		if blob, err := uc.BlobRepository.GetBlob(ctx, uc.OffloadingBucketUrl, decisionEvaluationKey); err == nil {
			defer blob.ReadCloser.Close()

			content, err := io.ReadAll(blob.ReadCloser)
			if err != nil {
				return err
			}

			ruleEvaluations, err := dbmodels.DeserializeDecisionEvaluationDto(content)
			if err != nil {
				return err
			}

			for idx, eval := range ruleEvaluations {
				decision.RuleExecutions[idx].Evaluation = eval
			}

			return nil
		}
	}

	offloadingWatermark, err := uc.Repository.GetWatermark(ctx, exec, &orgId, models.WatermarkTypeDecisionRules)
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
		key := uc.Repository.GetOffloadedDecisionRuleKey(orgId, rule.DecisionId,
			rule.Rule.Id, rule.Outcome, decision.CreatedAt)

		blob, err := uc.BlobRepository.GetBlob(ctx, uc.OffloadingBucketUrl, key)
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
