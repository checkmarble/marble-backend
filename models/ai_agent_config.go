package models

import (
	"encoding/json"
	"fmt"
	"io/fs"
)

// AiAgentModelConfig represents the configuration for AI agent models
type AiAgentModelConfig struct {
	// Default model to use when no specific model is configured for a prompt
	DefaultModel string `json:"default_model"`

	// Model configurations for specific prompts
	PromptModels map[string]string `json:"prompt_models"`
}

// LoadAiAgentModelConfig loads the AI agent model configuration from AiAgentModelConfigFileName
// at the root of promptsFS. The default model is provided by the caller depends on the provider
// it uses. If promptsFS is nil (no prompts filesystem available), the default configuration is
// returned.
func LoadAiAgentModelConfig(promptsFS fs.FS, defaultModel string) (*AiAgentModelConfig, error) {
	if promptsFS == nil {
		// Return default configuration if no prompts filesystem is available
		return &AiAgentModelConfig{
			DefaultModel: defaultModel,
			PromptModels: make(map[string]string),
		}, nil
	}

	file, err := promptsFS.Open(AiAgentModelConfigFileName)
	if err != nil {
		return nil, fmt.Errorf("could not open AI agent config file %s: %w", AiAgentModelConfigFileName, err)
	}
	defer file.Close()

	var config AiAgentModelConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("could not decode AI agent config file %s: %w", AiAgentModelConfigFileName, err)
	}

	// Set default model if not specified
	if config.DefaultModel == "" {
		config.DefaultModel = defaultModel
	}

	// Initialize maps if they're nil
	if config.PromptModels == nil {
		config.PromptModels = make(map[string]string)
	}

	return &config, nil
}

// GetModelForPrompt returns the appropriate model for a given prompt path
func (c *AiAgentModelConfig) GetModelForPrompt(promptPath string) string {
	// First, check for exact prompt path match
	if model, exists := c.PromptModels[promptPath]; exists {
		return model
	}

	// Finally, return the default model
	return c.DefaultModel
}
