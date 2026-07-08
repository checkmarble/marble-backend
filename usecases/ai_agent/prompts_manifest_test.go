package ai_agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeAllExpectedPrompts(t *testing.T, dir string) {
	t.Helper()
	for _, path := range models.AiAgentExpectedFiles {
		full := filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("content"), 0o644))
	}
}

func Test_ValidatePromptsFS(t *testing.T) {
	t.Run("nil fs is reported as an error", func(t *testing.T) {
		err := ValidatePromptsFS(nil)
		assert.Error(t, err)
	})

	t.Run("all expected prompts present and readable passes", func(t *testing.T) {
		dir := t.TempDir()
		writeAllExpectedPrompts(t, dir)

		err := ValidatePromptsFS(os.DirFS(dir))
		assert.NoError(t, err)
	})

	t.Run("a missing prompt file is reported", func(t *testing.T) {
		dir := t.TempDir()
		writeAllExpectedPrompts(t, dir)
		require.NoError(t, os.Remove(filepath.Join(dir, SYSTEM_PROMPT_PATH)))

		err := ValidatePromptsFS(os.DirFS(dir))
		require.Error(t, err)
		assert.Contains(t, err.Error(), SYSTEM_PROMPT_PATH)
	})

	t.Run("empty fs reports every expected path as missing", func(t *testing.T) {
		err := ValidatePromptsFS(os.DirFS(t.TempDir()))
		require.Error(t, err)
		for _, path := range models.AiAgentExpectedFiles {
			assert.Contains(t, err.Error(), path)
		}
	})
}
