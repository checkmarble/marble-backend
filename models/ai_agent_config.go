package models

import (
	"encoding/json"
	"fmt"
	"os"
)

// AiAgentModelConfig represents the configuration for AI agent models
type AiAgentModelConfig struct {
	// Default model to use when no specific model is configured for a prompt
	DefaultModel string `json:"default_model"`

	// Model configurations for specific prompts
	PromptModels map[string]string `json:"prompt_models"`
}

// LoadAiAgentModelConfig loads the AI agent model configuration from a JSON file
// The default model is provided by the caller depends on the provider it uses
func LoadAiAgentModelConfig(configPath string, defaultModel string) (*AiAgentModelConfig, error) {
	if configPath == "" {
		// Return default configuration if no path is provided
		return &AiAgentModelConfig{
			DefaultModel: defaultModel,
			PromptModels: make(map[string]string),
		}, nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("could not open AI agent config file %s: %w", configPath, err)
	}
	defer file.Close()

	var config AiAgentModelConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("could not decode AI agent config file %s: %w", configPath, err)
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
