package ai_agent

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
)

func TestCheckSarGenerationConfigured(t *testing.T) {
	full := &models.AiAgentModelConfig{DefaultLightModel: "l", DefaultHeavyModel: "h"}
	tests := []struct {
		name     string
		provider infra.AIAgentProviderType
		models   *models.AiAgentModelConfig
		ok       bool
	}{
		{"configured", infra.AIAgentProviderTypeOpenAI, full, true},
		{"no provider", infra.AIAgentProviderTypeUnknown, full, false},
		{"empty provider", "", full, false},
		{"no model config", infra.AIAgentProviderTypeAIStudio, nil, false},
		{"no heavy model", infra.AIAgentProviderTypeAIStudio, &models.AiAgentModelConfig{DefaultLightModel: "l"}, false},
		{"heavy from feature override", infra.AIAgentProviderTypeAIStudio, &models.AiAgentModelConfig{
			DefaultLightModel: "l", SarGeneration: &models.FeatureModelOverride{HeavyModel: "h"},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &AiAgentUsecase{aiAgentModelConfig: tt.models, promptsFS: allSarPrompts()}
			uc.config.MainAgentProviderType = tt.provider
			err := uc.CheckSarGenerationConfigured()
			if tt.ok {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, models.BadParameterError)
		})
	}
}

func allSarPrompts() fs.FS {
	m := fstest.MapFS{}
	for _, name := range models.AiAgentOptionalFiles {
		m[name] = &fstest.MapFile{Data: []byte("p")}
	}
	return m
}

func TestCheckSarGenerationConfigured_MissingPrompts(t *testing.T) {
	cfg := &models.AiAgentModelConfig{DefaultLightModel: "l", DefaultHeavyModel: "h"}
	for name, fsys := range map[string]fs.FS{"nil fs": nil, "empty fs": fstest.MapFS{}, "one missing": func() fs.FS {
		m := allSarPrompts().(fstest.MapFS)
		delete(m, models.SarGenerationTracfinReviewPromptPath)
		return m
	}(), "understanding missing": func() fs.FS {
		m := allSarPrompts().(fstest.MapFS)
		delete(m, models.SarGenerationTracfinUnderstandingPromptPath)
		return m
	}()} {
		t.Run(name, func(t *testing.T) {
			uc := &AiAgentUsecase{aiAgentModelConfig: cfg, promptsFS: fsys}
			uc.config.MainAgentProviderType = infra.AIAgentProviderTypeOpenAI
			err := uc.CheckSarGenerationConfigured()
			require.ErrorIs(t, err, models.BadParameterError)
			require.Contains(t, err.Error(), "prompts de génération de déclaration absents")
		})
	}
}
