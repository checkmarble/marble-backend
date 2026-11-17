package ai_agent

import (
	"testing"

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
			uc := &AiAgentUsecase{aiAgentModelConfig: tt.models}
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
