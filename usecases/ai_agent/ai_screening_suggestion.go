package ai_agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const (
	PROMPT_SCREENING_HIT_EVALUATE_PATH   = "prompts/screening_hit_suggestion/evaluate_match.md"
	SCREENING_HIT_SYSTEM_PROMPT_PATH     = "prompts/screening_hit_suggestion/system.md"
	SCREENING_HIT_CONTEXT_PROMPT_PATH    = "prompts/screening_hit_suggestion/screening_context.md"
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

	// Build the screening context prompt once (static across all hits, enables context caching)
	language := aiSetting.CaseReviewSetting.Language
	contextPromptData := map[string]any{
		"TriggerObjectData": staticContext.triggerObjectData,
		"PivotData":         staticContext.pivotData,
		"ScreeningQuery":    staticContext.screeningQuery,
		"Language":          language,
	}
	_, screeningContextPrompt, err := uc.preparePromptWithModel(
		SCREENING_HIT_CONTEXT_PROMPT_PATH, contextPromptData)
	if err != nil {
		return errors.Wrap(err, "could not prepare screening context prompt")
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
			ctx, client, screening.Id, match, screeningContextPrompt, systemInstruction,
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
	screeningContextPrompt string,
	systemInstruction string,
) (agent_dto.AiScreeningHitSuggestionDto, error) {
	blobPath := screeningHitSuggestionBlobPath(screeningId, match.Id)
	logger := utils.LoggerFromContext(ctx)

	// Check if suggestion already exists (idempotency)
	existingSuggestion, err := uc.loadSuggestionFromBlob(ctx, blobPath)
	if err == nil {
		logger.DebugContext(ctx, "Suggestion already exists, skipping", "match_id", match.Id)
		return existingSuggestion, nil
	}
	if !errors.Is(err, models.NotFoundError) {
		return nil, errors.Wrap(err, "could not load suggestion from blob")
	}

	// Enrich match if not already enriched
	enrichedMatch := match
	if !match.Enriched {
		enriched, err := uc.screeningUsecase.EnrichMatchWithoutAuthorization(ctx, match.Id)
		if err != nil {
			logger.WarnContext(ctx, "Could not enrich match, using neutral suggestion",
				"match_id", match.Id, "error", err)
			return nil, errors.Wrap(err, "failed to enrich match")
		}
		enrichedMatch = enriched
	}

	// Prepare the per-match prompt data (only variable part)
	promptData := map[string]any{
		"MatchPayload": string(enrichedMatch.Payload),
		"MatchScore":   fmt.Sprintf("%.2f", enrichedMatch.Score),
	}

	model, matchPrompt, err := uc.preparePromptWithModel(PROMPT_SCREENING_HIT_EVALUATE_PATH, promptData)
	if err != nil {
		return nil, errors.Wrap(err, "could not prepare prompt")
	}

	logger.DebugContext(ctx, "Screening hit evaluation",
		"match_id", match.Id, "model", model)

	// Call LLM: static context first (cacheable), then per-match data
	response, err := llmberjack.NewRequest[screeningHitLlmOutput]().
		WithInstruction(systemInstruction).
		WithModel(model).
		WithText(llmberjack.RoleUser, screeningContextPrompt).
		WithText(llmberjack.RoleUser, matchPrompt).
		WithThinking(false).
		Do(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "LLM call failed")
	}

	llmOutput, err := response.Get(0)
	if err != nil {
		return nil, errors.Wrap(err, "could not get LLM response")
	}

	// Validate confidence level
	confidence := models.ScreeningHitConfidence(llmOutput.Confidence)
	if !confidence.IsValid() {
		logger.WarnContext(ctx, "Invalid confidence level from LLM, defaulting to neutral",
			"match_id", match.Id, "confidence", llmOutput.Confidence)
		confidence = models.ScreeningHitConfidenceNeutral
	}

	suggestion := agent_dto.ScreeningHitSuggestionV1{
		MatchId:    match.Id,
		EntityId:   match.EntityId,
		Confidence: confidence,
		Reason:     llmOutput.Reason,
		CreatedAt:  time.Now(),
	}

	// Write to blob storage
	if err := uc.writeSuggestionToBlob(ctx, blobPath, suggestion); err != nil {
		return nil, errors.Wrap(err, "could not write suggestion to blob")
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
) (agent_dto.AiScreeningHitSuggestionDto, error) {
	blob, err := uc.blobRepository.GetBlob(ctx, uc.caseManagerBucketUrl, blobPath)
	if err != nil {
		return nil, err
	}
	defer blob.ReadCloser.Close()

	var envelope agent_dto.ScreeningHitSuggestionBlob
	if err := json.NewDecoder(blob.ReadCloser).Decode(&envelope); err != nil {
		return nil, errors.Wrap(err, "could not decode suggestion blob")
	}

	return agent_dto.UnmarshalScreeningHitSuggestionDto(
		envelope.Version, bytes.NewReader(envelope.Content))
}

func (uc *AiAgentUsecase) writeSuggestionToBlob(
	ctx context.Context,
	blobPath string,
	dto agent_dto.AiScreeningHitSuggestionDto,
) error {
	envelope, err := agent_dto.NewScreeningHitSuggestionBlob(dto)
	if err != nil {
		return errors.Wrap(err, "could not create suggestion blob")
	}

	stream, err := uc.blobRepository.OpenStream(ctx, uc.caseManagerBucketUrl, blobPath, blobPath)
	if err != nil {
		return errors.Wrap(err, "could not open blob stream")
	}
	defer stream.Close()

	if err := json.NewEncoder(stream).Encode(envelope); err != nil {
		return errors.Wrap(err, "could not encode suggestion to blob")
	}
	return nil
}

// GetScreeningSuggestions retrieves all screening hit suggestions for a screening from blob storage.
func (uc *AiAgentUsecase) GetScreeningSuggestions(
	ctx context.Context,
	screeningId string,
) ([]agent_dto.AiScreeningHitSuggestionDto, error) {
	exec := uc.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	screening, err := uc.repository.GetScreening(ctx, exec, screeningId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get screening")
	}

	if err := uc.enforceCanReadScreeningSuggestions(ctx, exec, screening); err != nil {
		return nil, err
	}

	suggestions := make([]agent_dto.AiScreeningHitSuggestionDto, 0, len(screening.Matches))
	for _, match := range screening.Matches {
		blobPath := screeningHitSuggestionBlobPath(screeningId, match.Id)
		suggestion, err := uc.loadSuggestionFromBlob(ctx, blobPath)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				logger.DebugContext(ctx, "No suggestion found for match, skipping",
					"match_id", match.Id)
				continue
			}
			return nil, errors.Wrapf(err, "could not load suggestion for match %s", match.Id)
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
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

func (uc *AiAgentUsecase) enforceCanReadScreeningSuggestions(ctx context.Context,
	exec repositories.Executor, screening models.ScreeningWithMatches,
) error {
	decisions, err := uc.repository.DecisionsById(ctx, exec, []string{screening.DecisionId})
	if err != nil {
		return err
	}
	if len(decisions) == 0 {
		return errors.Wrap(models.NotFoundError,
			"could not find the decision linked to the screening")
	}

	return uc.enforceSecurityDecision.ReadDecision(decisions[0])
}
