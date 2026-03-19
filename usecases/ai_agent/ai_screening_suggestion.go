package ai_agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
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
	SCREENING_HIT_SUGGESTION_BLOB_PREFIX = "ai_screening_reviews"
)

func screeningHitSuggestionBlobPath(screeningId, matchId string) string {
	return fmt.Sprintf("%s/%s/%s.json", SCREENING_HIT_SUGGESTION_BLOB_PREFIX, screeningId, matchId)
}

type screeningHitLlmOutput struct {
	Confidence string `json:"confidence" jsonschema_description:"One of probable_false_positive, inconclusive, or investigate" jsonschema:"enum=probable_false_positive,enum=inconclusive,enum=investigate"`
	Reason     string `json:"reason" jsonschema_description:"Concise explanation of the suggestion (1-3 sentences)"`
}

func (uc *AiAgentUsecase) hasScreeningHitSuggestionEnabled(ctx context.Context, orgId uuid.UUID) (bool, error) {
	featureAccess, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return false, err
	}
	return featureAccess.CaseAiAssist.IsAllowed(), nil
}

// AnalyseScreeningHits processes all pending matches in a screening, calling the LLM for each
// match sequentially, and stores results in blob storage.
func (uc *AiAgentUsecase) AnalyseScreeningHits(ctx context.Context, screeningId string) error {
	logger := utils.LoggerFromContext(ctx)
	exec := uc.executorFactory.NewExecutor()

	screening, err := uc.repository.GetScreening(ctx, exec, screeningId)
	if err != nil {
		return errors.Wrap(err, "could not get screening")
	}

	if err := uc.enforceCanGenerateScreeningSuggestions(ctx, exec, screening.Screening); err != nil {
		return err
	}

	// Check feature access
	enabled, err := uc.hasScreeningHitSuggestionEnabled(ctx, screening.OrgId)
	if err != nil {
		return errors.Wrap(err, "could not check feature access")
	}
	if !enabled {
		return errors.Wrap(models.ForbiddenError, "AI screening hit suggestion is not enabled")
	}

	orgId := screening.OrgId

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

	// Build prompt data (static context shared across all hits)
	language := aiSetting.CaseReviewSetting.Language
	promptData := map[string]any{
		"ScreeningQuery": staticContext.screeningQuery,
		"Language":       language,
	}
	if len(staticContext.linkedObjects) > 0 {
		promptData["LinkedObjects"] = staticContext.linkedObjects
	}
	if len(staticContext.triggerObjectData) > 0 {
		promptData["TriggerObjectData"] = staticContext.triggerObjectData
	}
	if len(staticContext.pivotData) > 0 {
		promptData["PivotData"] = staticContext.pivotData
	}

	// Process each pending match sequentially.
	// On failure, continue processing remaining matches but return an error at the end
	// so the job is retried. Already-generated suggestions are skipped on retry (idempotent).
	suggestionsGenerated := 0
	var firstErr error
	for _, match := range pendingMatches {
		_, err := uc.analyseScreeningMatch(
			ctx, client, screening.Id, match, promptData, systemInstruction,
		)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to analyse screening match",
				"match_id", match.Id, "error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		suggestionsGenerated++
	}

	logger.InfoContext(ctx, "Completed screening hit suggestions",
		"screening_id", screeningId,
		"total_pending", len(pendingMatches),
		"suggestions_generated", suggestionsGenerated,
	)

	if firstErr != nil {
		return errors.Wrapf(firstErr,
			"failed to generate suggestions for some matches (%d/%d succeeded)",
			suggestionsGenerated, len(pendingMatches))
	}

	return nil
}

func (uc *AiAgentUsecase) analyseScreeningMatch(
	ctx context.Context,
	client *llmberjack.Llmberjack,
	screeningId string,
	match models.ScreeningMatch,
	promptData map[string]any,
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
			logger.WarnContext(ctx, "Could not enrich match",
				"match_id", match.Id, "error", err)
			return nil, errors.Wrap(err, "failed to enrich match")
		}
		enrichedMatch = enriched
	}

	// Add per-match data to the prompt data
	matchPromptData := make(map[string]any, len(promptData)+2)
	maps.Copy(matchPromptData, promptData)
	matchPromptData["MatchPayload"] = string(enrichedMatch.Payload)
	matchPromptData["MatchScore"] = fmt.Sprintf("%.2f", enrichedMatch.GetScoreFromPayload())

	_, model, userMessage, err := uc.preparePromptWithModel(PROMPT_SCREENING_HIT_EVALUATE_PATH, matchPromptData)
	if err != nil {
		return nil, errors.Wrap(err, "could not prepare prompt")
	}

	logger.DebugContext(ctx, "Screening hit evaluation",
		"match_id", match.Id, "model", model)

	// Call LLM
	response, err := llmberjack.NewRequest[screeningHitLlmOutput]().
		WithInstruction(systemInstruction).
		WithModel(model).
		WithText(llmberjack.RoleUser, userMessage).
		WithThinking(false).
		Do(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "LLM call failed")
	}

	llmOutput, err := response.Get(0)
	if err != nil {
		return nil, errors.Wrap(err, "could not get LLM response")
	}

	logger.DebugContext(ctx, "####### Screening suggestion LLM input and output ##########")
	logger.DebugContext(ctx, "System instruction", "instruction", systemInstruction)
	logger.DebugContext(ctx, "LLM output", "confidence", llmOutput.Confidence, "reason", llmOutput.Reason)
	logger.DebugContext(ctx, "##############################################################")

	// Validate confidence level
	confidence := models.ScreeningHitConfidence(llmOutput.Confidence)
	if !confidence.IsValid() {
		logger.WarnContext(ctx, "Invalid confidence level from LLM, defaulting to inconclusive",
			"match_id", match.Id, "confidence", llmOutput.Confidence)
		confidence = models.ScreeningHitConfidenceInconclusive
	}

	suggestion := agent_dto.ScreeningHitSuggestionV1{
		MatchId:    match.Id,
		EntityId:   match.EntityId,
		Confidence: confidence,
		Reason:     llmOutput.Reason,
		Version:    agent_dto.VersionScreeningHitSuggestionV1,
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
	linkedObjects     agent_dto.LinkedObjects
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

	// Fetch the data model to resolve links from the trigger object and pivot tables
	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, decision.OrganizationId,
		models.DataModelReadOptions{}, true)
	if err != nil {
		logger.WarnContext(ctx, "could not fetch data model for screening context", "error", err)
		return result, nil
	}

	// Fetch linked objects from the trigger object table via LinksToSingle
	triggerLinked := uc.fetchLinkedSingleObjects(ctx, decision.OrganizationId,
		&dataModel, decision.ClientObject.TableName, decision.ClientObject.Data, agent_dto.LinkedObjectSourceTrigger)
	result.linkedObjects = append(result.linkedObjects, triggerLinked...)

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
			pivotObject := pivotObjects[0]
			result.pivotData = pivotObject.PivotObjectData.Data

			// Fetch linked objects from the pivot table via LinksToSingle
			pivotLinked := uc.fetchLinkedSingleObjects(ctx, decision.OrganizationId,
				&dataModel, pivotObject.PivotObjectName, pivotObject.PivotObjectData.Data, agent_dto.LinkedObjectSourcePivot)
			result.linkedObjects = append(result.linkedObjects, pivotLinked...)
		}
	}

	return result, nil
}

// fetchLinkedSingleObjects follows LinksToSingle from a source table and fetches
// each linked parent object.
func (uc *AiAgentUsecase) fetchLinkedSingleObjects(
	ctx context.Context,
	orgId uuid.UUID,
	dataModel *models.DataModel,
	sourceTableName string,
	sourceData map[string]any,
	sourceEntity agent_dto.LinkedObjectSource,
) agent_dto.LinkedObjects {
	logger := utils.LoggerFromContext(ctx)
	table, ok := dataModel.Tables[sourceTableName]
	if !ok {
		return nil
	}

	var result agent_dto.LinkedObjects

	for linkName, link := range table.LinksToSingle {
		childFieldValue, ok := sourceData[link.ChildFieldName]
		if !ok {
			continue
		}
		childFieldValueStr, ok := childFieldValue.(string)
		if !ok {
			continue
		}
		if childFieldValueStr == "" {
			continue
		}

		objects, err := uc.ingestedDataReader.GetIngestedObject(ctx,
			orgId, dataModel, link.ParentTableName, childFieldValueStr, link.ParentFieldName)
		if err != nil {
			logger.WarnContext(ctx, "could not fetch linked single object",
				"link_name", linkName,
				"parent_table", link.ParentTableName,
				"error", err)
			continue
		}

		if len(objects) > 0 {
			result = append(result, agent_dto.LinkedObject{
				SourceEntity: sourceEntity,
				LinkName:     linkName,
				TableName:    link.ParentTableName,
				Data:         objects[0].Data,
			})
		}
	}

	return result
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

	if err := uc.enforceCanReadScreeningSuggestions(ctx, exec, screening.Screening); err != nil {
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

	screening, err := uc.repository.GetScreeningWithoutMatches(ctx, exec, screeningId)
	if err != nil {
		return errors.Wrap(err, "could not get screening")
	}

	if err := uc.enforceCanGenerateScreeningSuggestions(ctx, exec, screening); err != nil {
		return err
	}

	// Check feature access
	enabled, err := uc.hasScreeningHitSuggestionEnabled(ctx, screening.OrgId)
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

// TODO: For now there is no dedicated permission for screening hit suggestions like case review
// We only check if the user had read access to decision and in the generation method we check for the feature access.
func (uc *AiAgentUsecase) enforceCanGenerateScreeningSuggestions(ctx context.Context,
	exec repositories.Executor, screening models.Screening,
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

func (uc *AiAgentUsecase) enforceCanReadScreeningSuggestions(ctx context.Context,
	exec repositories.Executor, screening models.Screening,
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
