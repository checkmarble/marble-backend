package repositories

import (
	"context"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"gocloud.dev/blob"
)

type offloadableRepository interface {
	GetOffloadedDecisionRuleKey(orgId uuid.UUID, decisionId, ruleId, outcome string, createdAt time.Time) string
	GetOffloadedDecisionEvaluationKey(orgId uuid.UUID, decision models.Decision) string
	GetScoreComputationEvaluationKey(ruleset models.ScoringRuleset, score models.ScoringScore) string
	GetOffloadedScreeningMatchKey(orgId uuid.UUID, screeningId, matchId string) string
	GetOffloadedContinuousScreeningMatchKey(orgId, continuousScreeningId, matchId uuid.UUID) string
	GetOffloadedContinuousScreeningEntityKey(orgId, continuousScreeningId uuid.UUID) string
	GetWatermark(
		ctx context.Context,
		exec Executor,
		orgId *uuid.UUID,
		watermarkType models.WatermarkType,
	) (*models.Watermark, error)
}

func (uc OffloadedReadWriter) IsOffloadingEnabled() bool {
	return uc.OffloadingBucketUrl != ""
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
	if !uc.IsOffloadingEnabled() {
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
	if !uc.IsOffloadingEnabled() {
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

func (uc OffloadedReadWriter) OffloadScoreComputation(
	ctx context.Context,
	ruleset models.ScoringRuleset,
	score models.ScoringScore,
	evaluation []byte,
) error {
	if !uc.IsOffloadingEnabled() {
		return nil
	}

	opts := blob.WriterOptions{}
	opts.BeforeWrite = func(asFunc func(any) bool) error {
		var gcsWriter *storage.Writer

		if asFunc(&gcsWriter) {
			gcsWriter.CustomTime = score.CreatedAt
			gcsWriter.ChunkSize = 0
		}

		return nil
	}

	wr, err := uc.BlobRepository.OpenStreamWithOptions(ctx,
		uc.OffloadingBucketUrl,
		uc.Repository.GetScoreComputationEvaluationKey(ruleset, score),
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

func (uc OffloadedReadWriter) GetOffloadedScoreComputation(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	ruleset models.ScoringRuleset,
	score models.ScoringScore,
) ([]*ast.NodeEvaluationDto, error) {
	if !uc.IsOffloadingEnabled() {
		return nil, nil
	}

	bucket, err := uc.BlobRepository.RawBucket(ctx, uc.OffloadingBucketUrl)
	if err != nil {
		return nil, err
	}

	scoreEvaluationKey := uc.Repository.GetScoreComputationEvaluationKey(ruleset, score)

	exists, err := bucket.Exists(ctx, scoreEvaluationKey)
	if err != nil {
		return nil, err
	}

	if exists {
		if blob, err := uc.BlobRepository.GetBlob(ctx, uc.OffloadingBucketUrl, scoreEvaluationKey); err == nil {
			defer blob.ReadCloser.Close()

			content, err := io.ReadAll(blob.ReadCloser)
			if err != nil {
				return nil, err
			}

			ruleEvaluations, err := dbmodels.DeserializeDecisionEvaluationDto(content)
			if err != nil {
				return nil, err
			}

			return ruleEvaluations, nil
		}
	}

	return nil, nil
}

// writePayload writes a raw payload to the given blob key, overwriting any existing object.
func (uc OffloadedReadWriter) writePayload(ctx context.Context, key string, payload []byte) error {
	wr, err := uc.BlobRepository.OpenStream(ctx, uc.OffloadingBucketUrl, key, key)
	if err != nil {
		return err
	}
	defer wr.Close()

	if _, err := wr.Write(payload); err != nil {
		return err
	}

	return nil
}

// readPayload reads a raw payload from the given blob key. It returns (nil, nil) when the object
// does not exist, so callers can fall back to the DB column without treating a miss as an error.
func (uc OffloadedReadWriter) readPayload(ctx context.Context, key string) ([]byte, error) {
	blob, err := uc.BlobRepository.GetBlob(ctx, uc.OffloadingBucketUrl, key)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return nil, nil
		}
		return nil, err
	}
	defer blob.ReadCloser.Close()

	return io.ReadAll(blob.ReadCloser)
}

// OffloadScreeningMatchPayload writes a screening match payload to blob storage at its
// deterministic key. No-op when offloading is disabled.
func (uc OffloadedReadWriter) OffloadScreeningMatchPayload(
	ctx context.Context, orgId uuid.UUID, screeningId, matchId string, payload []byte,
) error {
	if !uc.IsOffloadingEnabled() {
		return nil
	}
	return uc.writePayload(ctx, uc.Repository.GetOffloadedScreeningMatchKey(orgId, screeningId, matchId), payload)
}

// ReadOffloadedScreeningMatchPayload reads a screening match payload from blob storage. Returns
// (nil, nil) when offloading is disabled or the object is missing.
func (uc OffloadedReadWriter) ReadOffloadedScreeningMatchPayload(
	ctx context.Context, orgId uuid.UUID, screeningId, matchId string,
) ([]byte, error) {
	if !uc.IsOffloadingEnabled() {
		return nil, nil
	}
	return uc.readPayload(ctx, uc.Repository.GetOffloadedScreeningMatchKey(orgId, screeningId, matchId))
}

// OffloadScreeningMatches writes every match payload of a screening to blob storage and returns a
// copy of the matches with their payload blanked, ready to be inserted with an empty `payload`
// column. Match IDs are assigned here (and back-filled onto the input matches) so the blob keys
// line up with the rows the repository will create, while the caller keeps the original matches
// with their payloads intact for the API response.
//
// When offloading is disabled it is a no-op: the input matches are returned unchanged.
func (uc OffloadedReadWriter) OffloadScreeningMatches(
	ctx context.Context, screening models.ScreeningWithMatches,
) ([]models.ScreeningMatch, error) {
	if !uc.IsOffloadingEnabled() {
		return screening.Matches, nil
	}

	offloaded := make([]models.ScreeningMatch, len(screening.Matches))

	for i := range screening.Matches {
		if screening.Matches[i].Id == "" {
			screening.Matches[i].Id = pure_utils.NewId().String()
		}

		if err := uc.OffloadScreeningMatchPayload(ctx, screening.OrgId, screening.Id,
			screening.Matches[i].Id, screening.Matches[i].Payload); err != nil {
			return nil, err
		}

		offloaded[i] = screening.Matches[i]
		offloaded[i].Payload = nil
	}

	return offloaded, nil
}

// HydrateScreeningMatches fills in, from blob storage, the payload of every match whose DB column
// is empty (the offloaded-payload signal) across the given screenings. It is the read-side
// counterpart of OffloadScreeningMatches, for callers that read screenings outside ScreeningUsecase
// (e.g. the AI case review). No-op when offloading is disabled. A match whose payload is missing
// from both the column and blob storage is logged and left empty rather than failing the read.
func (uc OffloadedReadWriter) HydrateScreeningMatches(
	ctx context.Context, screenings []models.ScreeningWithMatches,
) error {
	if !uc.IsOffloadingEnabled() {
		return nil
	}

	for _, screening := range screenings {
		for i := range screening.Matches {
			if len(screening.Matches[i].Payload) > 0 {
				continue
			}

			payload, err := uc.ReadOffloadedScreeningMatchPayload(ctx, screening.OrgId,
				screening.Matches[i].ScreeningId, screening.Matches[i].Id)
			if err != nil {
				return err
			}

			if len(payload) == 0 {
				utils.LoggerFromContext(ctx).WarnContext(ctx,
					"screening match payload is missing from blob storage",
					"org_id", screening.OrgId,
					"screening_id", screening.Matches[i].ScreeningId,
					"match_id", screening.Matches[i].Id,
				)
				continue
			}

			screening.Matches[i].Payload = payload
		}
	}

	return nil
}

// OffloadContinuousScreeningMatchPayload writes a continuous screening match payload to blob
// storage. No-op when offloading is disabled.
func (uc OffloadedReadWriter) OffloadContinuousScreeningMatchPayload(
	ctx context.Context, orgId, continuousScreeningId, matchId uuid.UUID, payload []byte,
) error {
	if !uc.IsOffloadingEnabled() {
		return nil
	}
	return uc.writePayload(ctx,
		uc.Repository.GetOffloadedContinuousScreeningMatchKey(orgId, continuousScreeningId, matchId), payload)
}

// ReadOffloadedContinuousScreeningMatchPayload reads a continuous screening match payload from
// blob storage. Returns (nil, nil) when offloading is disabled or the object is missing.
func (uc OffloadedReadWriter) ReadOffloadedContinuousScreeningMatchPayload(
	ctx context.Context, orgId, continuousScreeningId, matchId uuid.UUID,
) ([]byte, error) {
	if !uc.IsOffloadingEnabled() {
		return nil, nil
	}
	return uc.readPayload(ctx,
		uc.Repository.GetOffloadedContinuousScreeningMatchKey(orgId, continuousScreeningId, matchId))
}

// OffloadContinuousScreeningEntityPayload writes the OpenSanctions entity payload attached to a
// continuous screening to blob storage. No-op when offloading is disabled.
func (uc OffloadedReadWriter) OffloadContinuousScreeningEntityPayload(
	ctx context.Context, orgId, continuousScreeningId uuid.UUID, payload []byte,
) error {
	if !uc.IsOffloadingEnabled() {
		return nil
	}
	return uc.writePayload(ctx,
		uc.Repository.GetOffloadedContinuousScreeningEntityKey(orgId, continuousScreeningId), payload)
}

// ReadOffloadedContinuousScreeningEntityPayload reads the continuous screening entity payload
// from blob storage. Returns (nil, nil) when offloading is disabled or the object is missing.
func (uc OffloadedReadWriter) ReadOffloadedContinuousScreeningEntityPayload(
	ctx context.Context, orgId, continuousScreeningId uuid.UUID,
) ([]byte, error) {
	if !uc.IsOffloadingEnabled() {
		return nil, nil
	}
	return uc.readPayload(ctx,
		uc.Repository.GetOffloadedContinuousScreeningEntityKey(orgId, continuousScreeningId))
}

// OffloadContinuousScreeningEntity offloads the OpenSanctions entity payload of a continuous
// screening to blob storage (keyed by the screening id) and returns the value to store in the
// `opensanction_entity_payload` column: nil once offloaded, or the payload unchanged when
// offloading is disabled (or there is no payload).
func (uc OffloadedReadWriter) OffloadContinuousScreeningEntity(
	ctx context.Context, orgId, continuousScreeningId uuid.UUID, payload []byte,
) ([]byte, error) {
	if !uc.IsOffloadingEnabled() || len(payload) == 0 {
		return payload, nil
	}

	if err := uc.OffloadContinuousScreeningEntityPayload(ctx, orgId, continuousScreeningId, payload); err != nil {
		return nil, errors.Wrap(err, "failed to offload continuous screening entity payload")
	}

	return nil, nil
}

// OffloadContinuousScreeningMatches offloads match payloads to blob storage and returns a copy of
// the matches with a pre-assigned id (matching its blob key) and a blanked payload, ready to be
// inserted with empty payload columns. When offloading is disabled the matches are returned
// unchanged.
func (uc OffloadedReadWriter) OffloadContinuousScreeningMatches(
	ctx context.Context, orgId, continuousScreeningId uuid.UUID, matches []models.ScreeningMatch,
) ([]models.ScreeningMatch, error) {
	if !uc.IsOffloadingEnabled() {
		return matches, nil
	}

	offloaded := make([]models.ScreeningMatch, len(matches))
	for i := range matches {
		match := matches[i]
		if match.Id == "" {
			match.Id = pure_utils.NewId().String()
		}

		matchId, err := uuid.Parse(match.Id)
		if err != nil {
			return nil, errors.Wrap(err, "invalid screening match id")
		}

		if err := uc.OffloadContinuousScreeningMatchPayload(ctx, orgId, continuousScreeningId,
			matchId, match.Payload); err != nil {
			return nil, errors.Wrap(err, "failed to offload continuous screening match payload")
		}

		match.Payload = nil
		offloaded[i] = match
	}

	return offloaded, nil
}

// HydrateContinuousScreeningEntity fills in, from blob storage, the entity payload of a
// dataset-triggered continuous screening when it was offloaded (empty DB column). No-op when
// offloading is disabled or the screening has no entity id. A missing blob is logged and left
// empty rather than failing the read.
func (uc OffloadedReadWriter) HydrateContinuousScreeningEntity(
	ctx context.Context, screening *models.ContinuousScreeningWithMatches,
) error {
	if !uc.IsOffloadingEnabled() {
		return nil
	}

	if screening.OpenSanctionEntityId == nil || len(screening.OpenSanctionEntityPayload) > 0 {
		return nil
	}

	payload, err := uc.ReadOffloadedContinuousScreeningEntityPayload(ctx, screening.OrgId, screening.Id)
	if err != nil {
		return errors.Wrap(err, "failed to read continuous screening entity payload")
	}
	if len(payload) == 0 {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"continuous screening entity payload is missing from both the DB column and blob storage",
			"org_id", screening.OrgId,
			"continuous_screening_id", screening.Id,
		)
		return nil
	}

	screening.OpenSanctionEntityPayload = payload
	return nil
}

// HydrateContinuousScreeningMatch fills in, from blob storage, the match payloads that
// were offloaded (empty DB column) for the given screening. No-op when offloading is disabled.
// A missing blob is logged and left empty rather than failing the read.
func (uc OffloadedReadWriter) HydrateContinuousScreeningMatch(
	ctx context.Context, screening *models.ContinuousScreeningWithMatches,
) error {
	if !uc.IsOffloadingEnabled() {
		return nil
	}

	for j := range screening.Matches {
		if len(screening.Matches[j].Payload) > 0 {
			continue
		}

		payload, err := uc.ReadOffloadedContinuousScreeningMatchPayload(ctx, screening.OrgId,
			screening.Id, screening.Matches[j].Id)
		if err != nil {
			return errors.Wrap(err, "failed to read continuous screening match payload")
		}
		if len(payload) == 0 {
			utils.LoggerFromContext(ctx).WarnContext(ctx,
				"continuous screening match payload is missing from both the DB column and blob storage",
				"org_id", screening.OrgId,
				"continuous_screening_id", screening.Id,
				"match_id", screening.Matches[j].Id,
			)
			continue
		}

		screening.Matches[j].Payload = payload
	}

	return nil
}
