package infra

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
)

func TestZipToPromptsFS_LoadsOptionalFilesWhenPresent(t *testing.T) {
	require.NotEmpty(t, models.AiAgentOptionalFiles)
	require.Contains(t, models.AiAgentOptionalFiles, "sar_generation/tracfin/placement.md")
	for _, p := range models.AiAgentOptionalFiles {
		require.NotContains(t, models.AiAgentExpectedFiles, p, "an optional file is not an expected one")
	}

	files := make(map[string]string, len(models.AiAgentExpectedFiles)+len(models.AiAgentOptionalFiles))
	for _, p := range models.AiAgentExpectedFiles {
		files[p] = "content for " + p
	}
	for _, p := range models.AiAgentOptionalFiles {
		files[p] = "optional content for " + p
	}

	fsys, err := zipToPromptsFS(writePromptsZip(t, files))
	require.NoError(t, err)
	for _, p := range models.AiAgentOptionalFiles {
		assert.Equal(t, "optional content for "+p, readFileFromFS(t, fsys, p),
			"an optional file present in the archive is loaded")
	}
}

func TestZipToPromptsFS_MissingOptionalFileIsNotAnError(t *testing.T) {
	require.NotEmpty(t, models.AiAgentOptionalFiles)

	files := make(map[string]string, len(models.AiAgentExpectedFiles))
	for _, p := range models.AiAgentExpectedFiles {
		files[p] = "content for " + p
	}

	fsys, err := zipToPromptsFS(writePromptsZip(t, files))
	require.NoError(t, err, "an absent optional file is not an error")
	for _, p := range models.AiAgentOptionalFiles {
		_, err := fs.ReadFile(fsys, p)
		assert.ErrorIs(t, err, fs.ErrNotExist, "an absent optional file is simply not loaded")
	}
	for _, p := range models.AiAgentExpectedFiles {
		assert.Equal(t, "content for "+p, readFileFromFS(t, fsys, p))
	}
}
