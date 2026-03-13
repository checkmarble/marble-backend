package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const (
	PROMPT_SCREENING_HIT_EVALUATE_PATH   = "prompts/screening_hit_suggestion/evaluate_match.md"
	SCREENING_HIT_SYSTEM_PROMPT_PATH     = "prompts/screening_hit_suggestion/system.md"
	SCREENING_HIT_SUGGESTION_BLOB_PREFIX = "ai_screening_reviews"
)

func screeningHitSuggestionBlobPath(screeningId, matchId string) string {
	return fmt.Sprintf("%s/%s/%s.json", SCREENING_HIT_SUGGESTION_BLOB_PREFIX, screeningId, matchId)
}

type screeningHitLlmOutput struct {
	Confidence string `json:"confidence" jsonschema_description:"One of probable_false_positive, neutral, or investigate" jsonschema:"enum=probable_false_positive,enum=neutral,enum=investigate"`
	Reason     string `json:"reason" jsonschema_description:"Concise explanation of the suggestion (1-3 sentences)"`
}

func (uc *AiAgentUsecase) HasScreeningHitSuggestionEnabled(ctx context.Context, orgId uuid.UUID) (bool, error) {
	featureAccess, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return false, err
	}
	return featureAccess.CaseAiAssist.IsAllowed(), nil
}

// AnalyseScreeningHits processes all pending matches in a screening, calling the LLM for each
// match sequentially, and stores results in blob storage.
func (uc *AiAgentUsecase) AnalyseScreeningHits(
	ctx context.Context,
	screeningId string,
	orgId uuid.UUID,
) error {
	// Check feature access
	enabled, err := uc.HasScreeningHitSuggestionEnabled(ctx, orgId)
	if err != nil {
		return errors.Wrap(err, "could not check feature access")
	}
	if !enabled {
		return errors.Wrap(models.ForbiddenError,
			"AI screening hit suggestion is not enabled")
	}

	logger := utils.LoggerFromContext(ctx)
	exec := uc.executorFactory.NewExecutor()

	// Fetch screening with matches
	screening, err := uc.repository.GetScreening(ctx, exec, screeningId)
	if err != nil {
		return errors.Wrap(err, "could not get screening")
	}

	// Get AI setting for language preference
	aiSetting, err := uc.getAiSetting(ctx, orgId)
	if err != nil {
		return errors.Wrap(err, "could not get ai setting")
	}

	// Get LLM client
	client, err := uc.GetClient(ctx)
	if err != nil {
		return errors.Wrap(err, "could not create ai client")
	}

	// Read system prompt
	systemInstruction, err := readPrompt(SCREENING_HIT_SYSTEM_PROMPT_PATH)
	if err != nil {
		logger.DebugContext(ctx, "could not read screening hit system instruction", "error", err)
		systemInstruction = "You are a compliance screening analyst. Assess whether each screening hit is a true positive or false positive."
	}

	// Build static context (same for all hits in this screening)
	staticContext, err := uc.buildScreeningStaticContext(ctx, exec, screening)
	if err != nil {
		logger.WarnContext(ctx, "could not build screening static context", "error", err)
		return errors.Wrap(err, "could not build screening static context")
	}

	// Filter matches to only pending ones
	var pendingMatches []models.ScreeningMatch
	for _, match := range screening.Matches {
		if match.Status == models.ScreeningMatchStatusPending {
			pendingMatches = append(pendingMatches, match)
		}
	}

	if len(pendingMatches) == 0 {
		logger.InfoContext(ctx, "No pending matches to analyse", "screening_id", screeningId)
		return nil
	}

	// Process each pending match sequentially
	suggestionsGenerated := 0
	for _, match := range pendingMatches {
		_, err := uc.analyseScreeningMatch(
			ctx, client, screening.Id, match,
			staticContext, systemInstruction, aiSetting, logger,
		)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to analyse screening match, skipping",
				"match_id", match.Id, "error", err)
			continue
		}
		suggestionsGenerated++
	}

	logger.InfoContext(ctx, "Completed screening hit suggestions",
		"screening_id", screeningId,
		"total_pending", len(pendingMatches),
		"suggestions_generated", suggestionsGenerated,
	)

	return nil
}

func (uc *AiAgentUsecase) analyseScreeningMatch(
	ctx context.Context,
	client *llmberjack.Llmberjack,
	screeningId string,
	match models.ScreeningMatch,
	staticContext screeningStaticContext,
	systemInstruction string,
	aiSetting models.AiSetting,
	logger *slog.Logger,
) (models.AiScreeningHitSuggestion, error) {
	blobPath := screeningHitSuggestionBlobPath(screeningId, match.Id)

	// Check if suggestion already exists (idempotency)
	existingSuggestion, err := uc.loadSuggestionFromBlob(ctx, blobPath)
	if err == nil {
		logger.DebugContext(ctx, "Suggestion already exists, skipping", "match_id", match.Id)
		return existingSuggestion, nil
	}
	if !errors.Is(err, models.NotFoundError) {
		return models.AiScreeningHitSuggestion{},
			errors.Wrap(err, "could not load suggestion from blob")
	}

	// Enrich match if not already enriched
	enrichedMatch := match
	if !match.Enriched {
		enriched, err := uc.screeningUsecase.EnrichMatchWithoutAuthorization(ctx, match.Id)
		if err != nil {
			logger.WarnContext(ctx, "Could not enrich match, using neutral suggestion",
				"match_id", match.Id, "error", err)
			return models.AiScreeningHitSuggestion{},
				errors.Wrap(err, "failed to enrich match")
		}
		enrichedMatch = enriched
	}

	// Prepare the per-match prompt data
	language := aiSetting.CaseReviewSetting.Language
	promptData := map[string]any{
		"TriggerObjectData": staticContext.triggerObjectData,
		"PivotData":         staticContext.pivotData,
		"ScreeningQuery":    staticContext.screeningQuery,
		"MatchPayload":      string(enrichedMatch.Payload),
		"MatchScore":        fmt.Sprintf("%.2f", enrichedMatch.Score),
		"Language":          language,
	}

	model, prompt, err := uc.preparePromptWithModel(PROMPT_SCREENING_HIT_EVALUATE_PATH, promptData)
	if err != nil {
		return models.AiScreeningHitSuggestion{},
			errors.Wrap(err, "could not prepare prompt")
	}

	logger.DebugContext(ctx, "Screening hit evaluation",
		"match_id", match.Id, "model", model)

	// Call LLM
	response, err := llmberjack.NewRequest[screeningHitLlmOutput]().
		WithInstruction(systemInstruction).
		WithModel(model).
		WithText(llmberjack.RoleUser, prompt).
		WithThinking(false).
		Do(ctx, client)
	if err != nil {
		return models.AiScreeningHitSuggestion{}, errors.Wrap(err, "LLM call failed")
	}

	llmOutput, err := response.Get(0)
	if err != nil {
		return models.AiScreeningHitSuggestion{},
			errors.Wrap(err, "could not get LLM response")
	}

	// Validate confidence level
	confidence := models.ScreeningHitConfidence(llmOutput.Confidence)
	validConfidence := slices.Contains(models.ScreeningHitConfidenceLevels, confidence)
	if !validConfidence {
		logger.WarnContext(ctx, "Invalid confidence level from LLM, defaulting to neutral",
			"match_id", match.Id, "confidence", llmOutput.Confidence)
		confidence = models.ScreeningHitConfidenceNeutral
	}

	suggestion := models.AiScreeningHitSuggestion{
		MatchId:    match.Id,
		EntityId:   match.EntityId,
		Confidence: confidence,
		Reason:     llmOutput.Reason,
		CreatedAt:  time.Now(),
	}

	// Write to blob storage
	if err := uc.writeSuggestionToBlob(ctx, blobPath, suggestion); err != nil {
		return models.AiScreeningHitSuggestion{},
			errors.Wrap(err, "could not write suggestion to blob")
	}

	return suggestion, nil
}

type screeningStaticContext struct {
	triggerObjectData map[string]any
	pivotData         map[string]any
	screeningQuery    string
}

func (uc *AiAgentUsecase) buildScreeningStaticContext(
	ctx context.Context,
	exec repositories.Executor,
	screening models.ScreeningWithMatches,
) (screeningStaticContext, error) {
	logger := utils.LoggerFromContext(ctx)
	var result screeningStaticContext

	// SearchInput contains the query that was used for screening
	if screening.SearchInput != nil {
		result.screeningQuery = string(screening.SearchInput)
	}

	// InitialQuery contains the structured query details
	if len(screening.InitialQuery) > 0 {
		queryJSON, err := json.Marshal(screening.InitialQuery)
		if err == nil {
			result.screeningQuery = string(queryJSON)
		}
	}

	// Fetch the decision linked to this screening to get the trigger object and pivot data
	decisions, err := uc.repository.DecisionsById(ctx, exec, []string{screening.DecisionId})
	if err != nil {
		logger.WarnContext(ctx, "could not fetch decision for screening context",
			"decision_id", screening.DecisionId, "error", err)
		return result, nil
	}
	if len(decisions) == 0 {
		return result, nil
	}
	decision := decisions[0]

	// Trigger object data: the client entity that was evaluated by the decision
	if len(decision.ClientObject.Data) > 0 {
		result.triggerObjectData = decision.ClientObject.Data
	}

	// Pivot data: the related entity (e.g. customer) linked to this decision
	if decision.PivotId != nil && decision.PivotValue != nil {
		pivotValues := []models.PivotDataWithCount{{
			PivotId:    decision.PivotId.String(),
			PivotValue: *decision.PivotValue,
		}}
		pivotObjects, err := uc.ingestedDataReader.ReadPivotObjectsFromValues(
			ctx, decision.OrganizationId, pivotValues)
		if err != nil {
			logger.WarnContext(ctx, "could not fetch pivot data for screening context",
				"pivot_value", *decision.PivotValue, "error", err)
			return result, nil
		}
		if len(pivotObjects) > 0 && len(pivotObjects[0].PivotObjectData.Data) > 0 {
			result.pivotData = pivotObjects[0].PivotObjectData.Data
		}
	}

	return result, nil
}

func (uc *AiAgentUsecase) loadSuggestionFromBlob(
	ctx context.Context,
	blobPath string,
) (models.AiScreeningHitSuggestion, error) {
	blob, err := uc.blobRepository.GetBlob(ctx, uc.caseManagerBucketUrl, blobPath)
	if err != nil {
		return models.AiScreeningHitSuggestion{}, err
	}
	defer blob.ReadCloser.Close()

	var suggestion models.AiScreeningHitSuggestion
	if err := json.NewDecoder(blob.ReadCloser).Decode(&suggestion); err != nil {
		return models.AiScreeningHitSuggestion{},
			errors.Wrap(err, "could not decode suggestion")
	}

	return suggestion, nil
}

func (uc *AiAgentUsecase) writeSuggestionToBlob(
	ctx context.Context,
	blobPath string,
	suggestion models.AiScreeningHitSuggestion,
) error {
	stream, err := uc.blobRepository.OpenStream(ctx, uc.caseManagerBucketUrl, blobPath, blobPath)
	if err != nil {
		return errors.Wrap(err, "could not open blob stream")
	}
	defer stream.Close()

	if err := json.NewEncoder(stream).Encode(suggestion); err != nil {
		return errors.Wrap(err, "could not encode suggestion to blob")
	}
	return nil
}

// GetScreeningHitSuggestion retrieves a single screening hit suggestion from blob storage.
func (uc *AiAgentUsecase) GetScreeningHitSuggestion(
	ctx context.Context,
	matchId string,
) (models.AiScreeningHitSuggestion, error) {
	exec := uc.executorFactory.NewExecutor()

	match, err := uc.repository.GetScreeningMatch(ctx, exec, matchId)
	if err != nil {
		return models.AiScreeningHitSuggestion{},
			errors.Wrap(err, "could not get screening match")
	}

	if err := uc.enforceCanReadScreeningHitSuggestion(ctx, exec, match); err != nil {
		return models.AiScreeningHitSuggestion{}, err
	}

	blobPath := screeningHitSuggestionBlobPath(match.ScreeningId, matchId)
	suggestion, err := uc.loadSuggestionFromBlob(ctx, blobPath)
	if err != nil {
		return models.AiScreeningHitSuggestion{},
			errors.Wrap(err, "could not load suggestion")
	}

	return suggestion, nil
}

// EnqueueScreeningHitSuggestion validates the screening and enqueues an async job.
func (uc *AiAgentUsecase) EnqueueScreeningHitSuggestion(
	ctx context.Context,
	screeningId string,
) error {
	exec := uc.executorFactory.NewExecutor()

	// Fetch screening to validate it exists and get the org ID
	screening, err := uc.repository.GetScreening(ctx, exec, screeningId)
	if err != nil {
		return errors.Wrap(err, "could not get screening")
	}

	// Check feature access
	enabled, err := uc.HasScreeningHitSuggestionEnabled(ctx, screening.OrgId)
	if err != nil {
		return errors.Wrap(err, "could not check feature access")
	}
	if !enabled {
		return errors.Wrap(models.ForbiddenError, "AI screening hit suggestion is not enabled")
	}

	// Enqueue the job
	if err := uc.screeningHitSuggestionTaskEnqueuer.EnqueueScreeningHitSuggestionTask(
		ctx, screening.OrgId, screeningId,
	); err != nil {
		return errors.Wrap(err, "could not enqueue screening hit suggestion task")
	}

	return nil
}

func (uc *AiAgentUsecase) enforceCanReadScreeningHitSuggestion(ctx context.Context,
	exec repositories.Executor, match models.ScreeningMatch,
) error {
	screening, err := uc.repository.GetScreeningWithoutMatches(ctx, exec, match.ScreeningId)
	if err != nil {
		return errors.Wrap(err, "could not get screening")
	}
	decision, err := uc.repository.DecisionsById(ctx, exec, []string{screening.DecisionId})
	if err != nil {
		return err
	}
	if len(decision) == 0 {
		return errors.Wrap(models.NotFoundError,
			"could not find the decision linked to the screening")
	}
	if decision[0].Case == nil {
		return errors.Wrap(models.UnprocessableEntityError,
			"this screening is not linked to a case")
	}

	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec,
		decision[0].OrganizationId, false)
	if err != nil {
		return errors.Wrap(err, "could not retrieve organization inboxes")
	}

	inboxIds := pure_utils.Map(inboxes, func(inbox models.Inbox) uuid.UUID {
		return inbox.Id
	})

	if err := uc.enforceSecurityCase.ReadOrUpdateCase((*decision[0].Case).GetMetadata(), inboxIds); err != nil {
		return err
	}

	return nil
}
