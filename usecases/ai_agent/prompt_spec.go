package ai_agent

import "github.com/checkmarble/marble-backend/models"

// promptSpec bundles a prompt's file path with the feature and model tier it needs, so each
// prompt declares once, at its definition site, which model configuration governs it.
type promptSpec struct {
	Path    string
	Feature models.AiFeature
	Tier    models.AiModelTier
}
