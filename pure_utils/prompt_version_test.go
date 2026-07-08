package pure_utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ResolveBestPromptVersion(t *testing.T) {
	t.Run("exact match wins", func(t *testing.T) {
		best, err := ResolveBestPromptVersion([]string{"v1.0", "v1.1", "v2.0"}, "1.1")
		require.NoError(t, err)
		assert.Equal(t, "v1.1", best)
	})

	t.Run("backward search: nearest earlier available, never forward, can cross a major", func(t *testing.T) {
		best, err := ResolveBestPromptVersion([]string{"v1.0", "v1.1", "v2.0"}, "1.5")
		require.NoError(t, err)
		assert.Equal(t, "v1.1", best)
	})

	t.Run("request above everything resolves to the nearest earlier, no ceiling", func(t *testing.T) {
		best, err := ResolveBestPromptVersion([]string{"v1.0", "v1.1", "v2.0"}, "3.0")
		require.NoError(t, err)
		assert.Equal(t, "v2.0", best)
	})

	t.Run("request below everything resolves to nothing", func(t *testing.T) {
		best, err := ResolveBestPromptVersion([]string{"v1.0", "v1.1"}, "0.9")
		require.NoError(t, err)
		assert.Equal(t, "", best)
	})

	t.Run("empty candidate list resolves to nothing, not an error", func(t *testing.T) {
		best, err := ResolveBestPromptVersion(nil, "1.0")
		require.NoError(t, err)
		assert.Equal(t, "", best)
	})

	t.Run("non-version-looking entries are skipped, not fatal", func(t *testing.T) {
		best, err := ResolveBestPromptVersion([]string{"main", "latest", "v1.0", "notes.txt"}, "1.5")
		require.NoError(t, err)
		assert.Equal(t, "v1.0", best)
	})

	t.Run("malformed requested version is an error", func(t *testing.T) {
		_, err := ResolveBestPromptVersion([]string{"v1.0"}, "not-a-version")
		require.Error(t, err)
	})
}
