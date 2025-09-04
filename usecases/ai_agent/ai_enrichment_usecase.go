package ai_agent

import (
	"context"
	"encoding/json"

	llmberjack "github.com/checkmarble/llmberjack"
	"github.com/checkmarble/llmberjack/llms/openai"
	perplexity "github.com/checkmarble/llmberjack/llms/perplexity"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

const ENRICHMENT_DEFAULT_MODEL = "sonar-pro"

func (uc *AiAgentUsecase) getEnrichmentAdapter() (*llmberjack.Llmberjack, error) {
	if uc.enrichmentAdapter != nil {
		return uc.enrichmentAdapter, nil
	}

	if uc.config.PerplexityAPIKey == "" {
		return nil, errors.New("Perplexity API key is not configured")
	}

	opts := []openai.Opt{
		openai.WithApiKey(uc.config.PerplexityAPIKey),
	}

	provider, err := perplexity.New(opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Perplexity provider")
	}

	adapter, err := llmberjack.New(
		llmberjack.WithProvider("enrichment", provider),
		llmberjack.WithDefaultModel(ENRICHMENT_DEFAULT_MODEL),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create LLM adapter")
	}
	uc.enrichmentAdapter = adapter
	return uc.enrichmentAdapter, nil
}

type PivotDataForEnrichment struct {
	PivotObjectName string         `json:"pivot_object_name"`
	PivotData       map[string]any `json:"pivot_data"`
}

func (uc *AiAgentUsecase) EnrichCasePivotObjects(ctx context.Context, caseId string) ([]models.AiEnrichmentKYC, error) {
	logger := utils.LoggerFromContext(ctx)

	// Get case data, included pivot data
	caseData, _, err := uc.getCaseDataWithPermissions(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case with permissions")
	}

	pivotDataForEnrichment := make([]PivotDataForEnrichment, 0, len(caseData.pivotData))
	for _, pivotObject := range caseData.pivotData {
		pivotDataForEnrichment = append(pivotDataForEnrichment, PivotDataForEnrichment{
			PivotObjectName: pivotObject.PivotObjectName,
			PivotData:       pivotObject.PivotObjectData.Data,
		})
	}

	var results []models.AiEnrichmentKYC

	// For each pivot, do LLM request to enrich data
	// TODO: Parallelize this with ALL function from llmberjack
	for _, pivotObject := range pivotDataForEnrichment {
		logger.DebugContext(
			ctx,
			"KYC Enrichment - Pivot data for enrichment",
			"pivotDataForEnrichment",
			pivotObject,
		)
		pivotDataJson, err := json.Marshal(pivotObject)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal pivot data for enrichment")
		}
		result, err := uc.enrichData(ctx, caseData.organizationId, pivotDataJson)
		if err != nil {
			return nil, errors.Wrap(err, "failed to enrich data")
		}

		results = append(results, result)
	}

	return results, nil
}

// Function to launch LLM request to enrich data
// data is JSON encoded which contains the data information like people name or company name
func (uc *AiAgentUsecase) enrichData(ctx context.Context, organizationId string, data []byte) (models.AiEnrichmentKYC, error) {
	logger := utils.LoggerFromContext(ctx)
	exec := uc.executorFactory.NewExecutor()

	// Get setting
	aiSetting, err := uc.repository.GetAiSetting(ctx, exec, organizationId)
	if err != nil {
		return models.AiEnrichmentKYC{}, errors.Wrap(err, "could not retrieve ai setting")
	}

	// Get or initialize the enrichment adapter
	adapter, err := uc.getEnrichmentAdapter()
	if err != nil {
		return models.AiEnrichmentKYC{}, errors.Wrap(err, "failed to get enrichment adapter")
	}

	instruction, err := readPrompt("prompts/kyc_enrichment/instruction.md")
	if err != nil {
		return models.AiEnrichmentKYC{}, errors.Wrap(err, "failed to read instruction")
	}
	_, prompt, err := uc.prepareRequest("prompts/kyc_enrichment/prompt_enrich.md", map[string]any{
		"data": string(data),
	})

	logger.DebugContext(ctx, "KYC Enrichment - Prompt", "prompt", prompt)

	type output struct {
		Analysis   string `json:"analysis" jsonschema_description:"The complete, unmodified analysis with all details, formatting, and citations preserved exactly as you would normally provide"`
		EntityName string `json:"entity_name" jsonschema_description:"The name of the main entity analyzed"`
	}

	request := llmberjack.NewRequest[output]().
		WithInstruction(instruction).
		WithText(llmberjack.RoleUser, prompt)

	// Override the default model if set in the AI setting
	if aiSetting.KYCEnrichmentSetting != nil && aiSetting.KYCEnrichmentSetting.Model != nil {
		request.WithModel(*aiSetting.KYCEnrichmentSetting.Model)
	}

	response, err := request.Do(ctx, adapter)
	if err != nil {
		return models.AiEnrichmentKYC{}, errors.Wrap(err, "failed to make request")
	}
	logger.DebugContext(ctx, "KYC Enrichment - Response", "response", response)

	var grounding []llmberjack.ResponseGroundingSource
	if len(response.Candidates) > 0 && response.Candidates[0].Grounding != nil {
		grounding = response.Candidates[0].Grounding.Sources
	}
	responseString, err := response.Get(0)
	if err != nil {
		return models.AiEnrichmentKYC{}, errors.Wrap(err, "failed to get response")
	}
	logger.DebugContext(ctx, "KYC Enrichment - Response", "responseString", responseString)

	return models.AiEnrichmentKYC{
		Analysis:   responseString.Analysis,
		EntityName: responseString.EntityName,
		Citations: pure_utils.Map(grounding, func(source llmberjack.ResponseGroundingSource) models.Citation {
			return models.Citation{
				Title:  source.Title,
				Domain: source.Domain,
				Url:    source.Url,
				Date:   source.Date,
			}
		}),
	}, nil
}
