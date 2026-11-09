package models

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func writeAiAgentConfigOverrideFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// Without any override, the config is similar to the base config we can have in marble prompts.
func TestLoadAiAgentModelConfig_FromPromptsFS(t *testing.T) {
	promptsFS := fstest.MapFS{
		AiAgentModelConfigFileName: {Data: []byte(`{
			"default_light_model": "gemini-2.5-flash",
			"default_heavy_model": "gemini-2.5-pro",
			"rule_builder": { "heavy_model": "claude-opus-4-6" }
		}`)},
	}

	config, err := LoadAiAgentModelConfig(promptsFS, "")
	require.NoError(t, err)

	require.Equal(t, "gemini-2.5-flash", config.GetModel(AiFeatureCaseReview, AiModelTierLight))
	require.Equal(t, "gemini-2.5-pro", config.GetModel(AiFeatureCaseReview, AiModelTierHeavy))
	require.Equal(t, "gemini-2.5-flash", config.GetModel(AiFeatureRuleDescription, AiModelTierLight))
	require.Equal(t, "gemini-2.5-flash", config.GetModel(AiFeatureRuleBuilder, AiModelTierLight))
	require.Equal(t, "claude-opus-4-6", config.GetModel(AiFeatureRuleBuilder, AiModelTierHeavy))
}

func TestLoadAiAgentModelConfig_OverridePartialGlobalDefaults(t *testing.T) {
	promptsFS := fstest.MapFS{
		AiAgentModelConfigFileName: {Data: []byte(`{
			"default_light_model": "gemini-2.5-flash",
			"default_heavy_model": "gemini-2.5-pro"
		}`)},
	}
	overridePath := writeAiAgentConfigOverrideFile(t, t.TempDir(), "override.json", `{
		"default_light_model": "custom-light-model"
	}`)

	config, err := LoadAiAgentModelConfig(promptsFS, overridePath)
	require.NoError(t, err)

	require.Equal(t, "custom-light-model", config.GetModel(AiFeatureCaseReview, AiModelTierLight))
	require.Equal(t, "gemini-2.5-pro", config.GetModel(AiFeatureCaseReview, AiModelTierHeavy))
}

func TestLoadAiAgentModelConfig_OverridePartialFeatureField(t *testing.T) {
	promptsFS := fstest.MapFS{
		AiAgentModelConfigFileName: {Data: []byte(`{
			"default_light_model": "gemini-2.5-flash",
			"default_heavy_model": "gemini-2.5-pro",
			"rule_builder": { "heavy_model": "claude-opus-4-6" }
		}`)},
	}
	overridePath := writeAiAgentConfigOverrideFile(t, t.TempDir(), "override.json", `{
		"rule_builder": { "light_model": "custom-light-model" }
	}`)

	config, err := LoadAiAgentModelConfig(promptsFS, overridePath)
	require.NoError(t, err)

	require.Equal(t, "custom-light-model", config.GetModel(AiFeatureRuleBuilder, AiModelTierLight))
	require.Equal(t, "claude-opus-4-6", config.GetModel(AiFeatureRuleBuilder, AiModelTierHeavy))
	require.Equal(t, "gemini-2.5-flash", config.GetModel(AiFeatureCaseReview, AiModelTierLight))
	require.Equal(t, "gemini-2.5-pro", config.GetModel(AiFeatureCaseReview, AiModelTierHeavy))
}

func TestLoadAiAgentModelConfig_NoPromptsFS_ReturnsError(t *testing.T) {
	_, err := LoadAiAgentModelConfig(nil, "")
	require.Error(t, err)
}

func TestLoadAiAgentModelConfig_MissingDefaults_ReturnsError(t *testing.T) {
	promptsFS := fstest.MapFS{
		AiAgentModelConfigFileName: {Data: []byte(`{
			"rule_builder": { "heavy_model": "claude-opus-4-6" }
		}`)},
	}

	_, err := LoadAiAgentModelConfig(promptsFS, "")
	require.Error(t, err)
}

func TestLoadAiAgentModelConfig_NoPromptsFSWithOverride_ProvidingBothDefaults(t *testing.T) {
	overridePath := writeAiAgentConfigOverrideFile(t, t.TempDir(), "override.json", `{
		"default_light_model": "gemini-2.5-flash",
		"default_heavy_model": "claude-opus-4-6"
	}`)

	config, err := LoadAiAgentModelConfig(nil, overridePath)
	require.NoError(t, err)

	require.Equal(t, "gemini-2.5-flash", config.GetModel(AiFeatureCaseReview, AiModelTierLight))
	require.Equal(t, "claude-opus-4-6", config.GetModel(AiFeatureCaseReview, AiModelTierHeavy))
}

func TestLoadAiAgentModelConfig_OverrideFeatureNotInBase(t *testing.T) {
	promptsFS := fstest.MapFS{
		AiAgentModelConfigFileName: {Data: []byte(`{
			"default_light_model": "gemini-2.5-flash",
			"default_heavy_model": "gemini-2.5-pro"
		}`)},
	}
	overridePath := writeAiAgentConfigOverrideFile(t, t.TempDir(), "override.json", `{
		"screening_hit_suggestion": { "light_model": "custom-light-model" }
	}`)

	config, err := LoadAiAgentModelConfig(promptsFS, overridePath)
	require.NoError(t, err)

	require.Equal(t, "custom-light-model", config.GetModel(AiFeatureScreeningHitSuggestion, AiModelTierLight))
	require.Equal(t, "gemini-2.5-pro", config.GetModel(AiFeatureScreeningHitSuggestion, AiModelTierHeavy))
}

func TestLoadAiAgentModelConfig_TracfinOverrideDoesNotChangeOtherFeatures(t *testing.T) {
	promptsFS := fstest.MapFS{AiAgentModelConfigFileName: {Data: []byte(`{
  "default_light_model":"global-light", "default_heavy_model":"global-heavy",
  "tracfin":{"light_model":"bundle-model"},
  "rule_builder":{"light_model":"rule-light", "heavy_model":"rule-heavy"}
 }`)}}
	for _, tc := range []struct{ name, override, want string }{
		{"flash", `{"tracfin":{"light_model":"gemini-3.5-flash"}}`, "gemini-3.5-flash"},
		{"other gemini", `{"tracfin":{"light_model":"gemini-2.5-flash"}}`, "gemini-2.5-flash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeAiAgentConfigOverrideFile(t, t.TempDir(), "override.json", tc.override)
			config, err := LoadAiAgentModelConfig(promptsFS, path)
			require.NoError(t, err)
			require.Equal(t, tc.want, config.GetModel(AiFeatureTracfin, AiModelTierLight))
			require.Equal(t, "global-light", config.DefaultLightModel)
			require.Equal(t, "global-heavy", config.DefaultHeavyModel)
			for _, feature := range []AiFeature{AiFeatureCaseReview, AiFeatureRuleDescription, AiFeatureScreeningHitSuggestion} {
				require.Equal(t, "global-light", config.GetModel(feature, AiModelTierLight))
				require.Equal(t, "global-heavy", config.GetModel(feature, AiModelTierHeavy))
			}
			require.Equal(t, "rule-light", config.GetModel(AiFeatureRuleBuilder, AiModelTierLight))
			require.Equal(t, "rule-heavy", config.GetModel(AiFeatureRuleBuilder, AiModelTierHeavy))
		})
	}
}
