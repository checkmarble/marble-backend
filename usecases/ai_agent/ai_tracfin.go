package ai_agent

import (
	"bytes"
	"context"
	"text/template"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/invopop/jsonschema"
)

type TracfinAIResponse struct {
	Text         string
	Provider     string
	Model        string
	FinishReason string
	Usage        llmberjack.ResponseUsage
}

func (uc *AiAgentUsecase) TracfinAIPrompts() (map[string]string, error) {
	prompts := make(map[string]string, len(models.AiAgentTracfinPromptPaths))
	for _, path := range models.AiAgentTracfinPromptPaths {
		content, err := uc.readPrompt(path)
		if err != nil {
			return nil, err
		}
		prompts[path] = content
	}
	return prompts, nil
}

func RenderTracfinPrompt(content string, data any) (string, error) {
	t, err := template.New("tracfin").Option("missingkey=error").Parse(content)
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	if err := t.Execute(&output, data); err != nil {
		return "", err
	}
	return output.String(), nil
}

func (uc *AiAgentUsecase) TracfinAIModel() (string, string, error) {
	if uc.aiAgentModelConfig == nil || uc.aiAgentModelConfig.Tracfin == nil || uc.aiAgentModelConfig.Tracfin.LightModel == "" {
		return "", "", errors.New("TRACFIN AI model configuration is not available")
	}
	model := uc.aiAgentModelConfig.GetModel(models.AiFeatureTracfin, models.AiModelTierLight)
	if model == "" {
		return "", "", errors.New("TRACFIN AI model is not configured")
	}
	if uc.config.MainAgentProviderType != infra.AIAgentProviderTypeAIStudio || providerForModel(model) != "main" {
		return "", "", errors.New("TRACFIN requires the AI Studio provider to enforce the call budget")
	}
	return providerForModel(model), model, nil
}

func (uc *AiAgentUsecase) TracfinAIComplete(ctx context.Context, system string, prompt string, schema jsonschema.Schema) (TracfinAIResponse, error) {
	provider, model, err := uc.TracfinAIModel()
	if err != nil {
		return TracfinAIResponse{}, err
	}
	return uc.TracfinAICompleteWithModel(ctx, system, prompt, schema, provider, model)
}

// TracfinAICompleteWithModel uses the model frozen when the run was started.
func (uc *AiAgentUsecase) TracfinAICompleteWithModel(ctx context.Context, system string, prompt string, schema jsonschema.Schema, provider string, model string) (TracfinAIResponse, error) {
	if uc.config.MainAgentProviderType != infra.AIAgentProviderTypeAIStudio || provider != "main" {
		return TracfinAIResponse{}, errors.New("TRACFIN requires the AI Studio provider to enforce the call budget")
	}
	client, err := uc.GetClient(ctx)
	if err != nil {
		return TracfinAIResponse{}, err
	}
	response, err := DoLLMRequest(ctx, client, llmberjack.NewUntypedRequest().
		WithProvider(provider).WithModel(model).WithMaxTokens(65536).
		WithInstruction(system).WithText(llmberjack.RoleUser, prompt).OverrideResponseSchema(schema))
	if err != nil {
		return TracfinAIResponse{}, err
	}
	text, err := response.Get(0)
	if err != nil {
		return TracfinAIResponse{}, err
	}
	candidate, err := response.Candidate(0)
	if err != nil {
		return TracfinAIResponse{}, err
	}
	return TracfinAIResponse{Text: text, Provider: provider, Model: model, FinishReason: string(candidate.FinishReason), Usage: response.Usage}, nil
}
