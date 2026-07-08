package models

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_AiAgentPromptsMapFS(t *testing.T) {
	fsys := AiAgentPromptsMapFS{
		"system.md":              []byte("system content"),
		"case_review/summary.md": []byte("summary content"),
	}

	t.Run("ReadFile returns the content of a present path", func(t *testing.T) {
		content, err := fsys.ReadFile("system.md")
		require.NoError(t, err)
		assert.Equal(t, "system content", string(content))
	})

	t.Run("ReadFile returns fs.ErrNotExist for a missing path", func(t *testing.T) {
		_, err := fsys.ReadFile("does/not/exist.md")
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("fs.ReadFile takes the ReadFileFS fast path", func(t *testing.T) {
		content, err := fs.ReadFile(fsys, "case_review/summary.md")
		require.NoError(t, err)
		assert.Equal(t, "summary content", string(content))
	})

	t.Run("Open returns a readable file with correct Stat size and name", func(t *testing.T) {
		f, err := fsys.Open("system.md")
		require.NoError(t, err)
		defer f.Close()

		info, err := f.Stat()
		require.NoError(t, err)
		assert.Equal(t, "system.md", info.Name())
		assert.Equal(t, int64(len("system content")), info.Size())
		assert.False(t, info.IsDir())

		buf := make([]byte, info.Size())
		n, err := f.Read(buf)
		require.NoError(t, err)
		assert.Equal(t, "system content", string(buf[:n]))
	})

	t.Run("Open returns fs.ErrNotExist for a missing path", func(t *testing.T) {
		_, err := fsys.Open("missing.md")
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("Open rejects an invalid fs path", func(t *testing.T) {
		_, err := fsys.Open("../escape.md")
		assert.ErrorIs(t, err, fs.ErrInvalid)
	})
}
