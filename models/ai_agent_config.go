package models

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"

	"dario.cat/mergo"
	"github.com/cockroachdb/errors"
)

type AiModelTier string

const (
	AiModelTierLight AiModelTier = "light"
	AiModelTierHeavy AiModelTier = "heavy"
)

type AiFeature string

const (
	AiFeatureCaseReview             AiFeature = "case_review"
	AiFeatureRuleDescription        AiFeature = "rule_description"
	AiFeatureRuleBuilder            AiFeature = "rule_builder"
	AiFeatureScreeningHitSuggestion AiFeature = "screening_hit_suggestion"
)

// FeatureModelOverride overrides the light and/or heavy model for a single feature. Both
// fields are optional: an empty one falls back to the config's global default for that tier.
type FeatureModelOverride struct {
	LightModel string `json:"light_model,omitempty"`
	HeavyModel string `json:"heavy_model,omitempty"`
}

// AiAgentModelConfig represents the configuration for AI agent models: a global light/heavy
// default, and an optional per-feature override of either tier. Each known feature is its own
// named, optional field (rather than a map) so the schema is self-documenting and there's no
// namespace-collision risk with client-supplied keys.
type AiAgentModelConfig struct {
	DefaultLightModel string `json:"default_light_model,omitempty"`
	DefaultHeavyModel string `json:"default_heavy_model,omitempty"`

	CaseReview             *FeatureModelOverride `json:"case_review,omitempty"`
	RuleDescription        *FeatureModelOverride `json:"rule_description,omitempty"`
	RuleBuilder            *FeatureModelOverride `json:"rule_builder,omitempty"`
	ScreeningHitSuggestion *FeatureModelOverride `json:"screening_hit_suggestion,omitempty"`
}

func (c *AiAgentModelConfig) featureOverride(feature AiFeature) *FeatureModelOverride {
	switch feature {
	case AiFeatureCaseReview:
		return c.CaseReview
	case AiFeatureRuleDescription:
		return c.RuleDescription
	case AiFeatureRuleBuilder:
		return c.RuleBuilder
	case AiFeatureScreeningHitSuggestion:
		return c.ScreeningHitSuggestion
	default:
		return nil
	}
}

// Validate checks that c has enough information to resolve a model for every feature: both
// global defaults must be set, since any feature/tier without its own override falls back to
// them. Without this, GetModel could silently return "" for an unconfigured feature/tier.
func (c *AiAgentModelConfig) Validate() error {
	if c.DefaultLightModel == "" || c.DefaultHeavyModel == "" {
		return errors.New("ai agent model configuration is missing default_light_model and/or default_heavy_model")
	}
	return nil
}

// GetModel returns the model to use for feature at tier: the feature's override for that tier
// if set, else the config's global default for that tier.
func (c *AiAgentModelConfig) GetModel(feature AiFeature, tier AiModelTier) string {
	if override := c.featureOverride(feature); override != nil {
		switch tier {
		case AiModelTierLight:
			if override.LightModel != "" {
				return override.LightModel
			}
		case AiModelTierHeavy:
			if override.HeavyModel != "" {
				return override.HeavyModel
			}
		}
	}
	if tier == AiModelTierHeavy {
		return c.DefaultHeavyModel
	}
	return c.DefaultLightModel
}

func decodeAiAgentModelConfig(r io.Reader) (*AiAgentModelConfig, error) {
	var config AiAgentModelConfig
	if err := json.NewDecoder(r).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// loadAiAgentModelConfigFromFS reads and decodes AiAgentModelConfigFileName from promptsFS.
func loadAiAgentModelConfigFromFS(promptsFS fs.FS) (*AiAgentModelConfig, error) {
	file, err := promptsFS.Open(AiAgentModelConfigFileName)
	if err != nil {
		return nil, errors.Newf("could not open AI agent config file %s: %w", AiAgentModelConfigFileName, err)
	}
	defer file.Close()

	config, err := decodeAiAgentModelConfig(file)
	if err != nil {
		return nil, errors.Newf("could not decode AI agent config file %s: %w", AiAgentModelConfigFileName, err)
	}
	return config, nil
}

// loadAiAgentModelConfigFile reads and decodes an override config file from the local disk (not
// from promptsFS - the override is a deployment-local file, independent of wherever the base
// prompts/config came from).
func loadAiAgentModelConfigFile(path string) (*AiAgentModelConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Newf("could not open AI agent config override file %s: %w", path, err)
	}
	defer file.Close()

	config, err := decodeAiAgentModelConfig(file)
	if err != nil {
		return nil, errors.Newf("could not decode AI agent config override file %s: %w", path, err)
	}
	return config, nil
}

// LoadAiAgentModelConfig loads the AI agent model configuration.
// The base configuration comes from AiAgentModelConfigFileName at the root of promptsFS
// overridePath, if non-empty, points to an optional local JSON file (AI_AGENT_MODELS_CONFIG_OVERRIDE_FILE)
func LoadAiAgentModelConfig(promptsFS fs.FS, overridePath string) (*AiAgentModelConfig, error) {
	var base *AiAgentModelConfig
	if promptsFS == nil {
		base = &AiAgentModelConfig{}
	} else {
		var err error
		base, err = loadAiAgentModelConfigFromFS(promptsFS)
		if err != nil {
			return nil, err
		}
	}

	if overridePath != "" {
		override, err := loadAiAgentModelConfigFile(overridePath)
		if err != nil {
			return nil, errors.Newf("could not load AI agent config override file %s: %w", overridePath, err)
		}
		if err := mergo.Merge(base, override, mergo.WithOverride); err != nil {
			return nil, errors.Newf("could not merge AI agent config override file %s: %w", overridePath, err)
		}
	}

	if err := base.Validate(); err != nil {
		return nil, err
	}

	return base, nil
}
