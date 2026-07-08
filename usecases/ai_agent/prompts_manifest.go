package ai_agent

import (
	"io/fs"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

// Validate the prompts for the AI agent against the expected files used by AI usecases.
func ValidatePromptsFS(promptsFS fs.FS) error {
	if promptsFS == nil {
		return errors.New("ai prompts filesystem is not available")
	}

	var missing []string
	for _, path := range models.AiAgentExpectedFiles {
		if _, err := fs.ReadFile(promptsFS, path); err != nil {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		return errors.Newf("missing or unreadable ai prompt files: %v", missing)
	}
	return nil
}
