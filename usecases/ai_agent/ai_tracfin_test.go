package ai_agent

import (
	"encoding/json"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"testing/fstest"
)

func TestTracfinPromptsFreezeAndRender(t *testing.T) {
	files := fstest.MapFS{}
	for _, name := range []string{"tracfin/system.md", "tracfin/sections/persons.md", "tracfin/sections/attachments.md", "tracfin/sections/operations.md", "tracfin/sections/declaration.md", "tracfin/repair_section.md", "tracfin/review.md"} {
		files[name] = &fstest.MapFile{Data: []byte("input={{.Input}}")}
	}
	uc := &AiAgentUsecase{promptsFS: files}
	prompts, err := uc.TracfinAIPrompts()
	require.NoError(t, err)
	files["tracfin/system.md"].Data = []byte("changed")
	output, err := RenderTracfinPrompt(prompts["tracfin/system.md"], map[string]any{"Input": `{"a":"<x>"}`})
	require.NoError(t, err)
	require.Equal(t, `input={"a":"<x>"}`, output)
	delete(files, "tracfin/review.md")
	_, err = uc.TracfinAIPrompts()
	require.ErrorContains(t, err, "tracfin/review.md")
}

func TestProductionTracfinPromptTemplates(t *testing.T) {
	dir := os.Getenv("TRACFIN_PROMPTS_TEST_DIR")
	if dir == "" {
		t.Skip("set TRACFIN_PROMPTS_TEST_DIR to validate the production prompt bundle")
	}
	uc := &AiAgentUsecase{promptsFS: os.DirFS(dir)}
	prompts, err := uc.TracfinAIPrompts()
	require.NoError(t, err)
	input, err := json.Marshal(map[string]any{"section": "persons", "context": map[string]any{}, "analysis": "texte humain", "document": map[string]any{}, "catalog": map[string]any{}, "previous": tracfin.AISectionProposal{}, "diagnostics": []string{}, "findings": tracfin.AIReview{}})
	require.NoError(t, err)
	for name, prompt := range prompts {
		t.Run(name, func(t *testing.T) {
			rendered, err := RenderTracfinPrompt(prompt, map[string]any{"Input": string(input)})
			require.NoError(t, err)
			require.NotEmpty(t, rendered)
			if name != "tracfin/system.md" {
				require.Contains(t, rendered, string(input))
			}
		})
	}
}

func TestTracfinModelRequiresExplicitBoundedProvider(t *testing.T) {
	uc := &AiAgentUsecase{config: infra.AIAgentConfiguration{MainAgentProviderType: infra.AIAgentProviderTypeAIStudio}, aiAgentModelConfig: &models.AiAgentModelConfig{DefaultLightModel: "expensive-default"}}
	_, _, err := uc.TracfinAIModel()
	require.Error(t, err)
	uc.aiAgentModelConfig.Tracfin = &models.FeatureModelOverride{LightModel: "gemini-3.5-flash"}
	provider, model, err := uc.TracfinAIModel()
	require.NoError(t, err)
	require.Equal(t, "main", provider)
	require.Equal(t, "gemini-3.5-flash", model)
	uc.config.MainAgentProviderType = infra.AIAgentProviderTypeOpenAI
	_, _, err = uc.TracfinAIModel()
	require.Error(t, err)
}
