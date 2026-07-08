package infra

import (
	"archive/zip"
	"bytes"
	"context"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/avast/retry-go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
)

// failIfHitServer returns an httptest.Server that fails the test if it's ever hit - used to
// assert a tier that should short-circuit before the network never actually calls out.
func failIfHitServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected network call to %s", r.URL.String())
		w.WriteHeader(http.StatusInternalServerError)
	}))
}

// useAiPromptsServerURL points aiPromptsServerURLDefault at an httptest.Server for the
// duration of the test, restoring the original value afterwards.
func useAiPromptsServerURL(t *testing.T, url string) {
	t.Helper()
	original := aiPromptsServerURLDefault
	aiPromptsServerURLDefault = url
	t.Cleanup(func() { aiPromptsServerURLDefault = original })
}

func writePromptsZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := zw.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func readFileFromFS(t *testing.T, fsys fs.FS, name string) string {
	t.Helper()
	b, err := fs.ReadFile(fsys, name)
	require.NoError(t, err)
	return string(b)
}

// writeFullPromptsZip builds a zip archive containing every path in the expected-prompts
// manifest, so a tier-3 test can exercise a download that passes zipToPromptsFS's
// all-expected-files-present verification. overrides lets a test control the content of
// specific paths (e.g. models.AiAgentSystemPromptPath) to assert on afterwards.
func writeFullPromptsZip(t *testing.T, overrides map[string]string) []byte {
	t.Helper()
	files := make(map[string]string, len(models.AiAgentExpectedFiles))
	for _, path := range models.AiAgentExpectedFiles {
		files[path] = "content for " + path
	}
	maps.Copy(files, overrides)
	return writePromptsZip(t, files)
}

func Test_InitAiPromptsFS_Tier1_ExecutionDir(t *testing.T) {
	t.Run("execution dir set and readable is used as-is", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "system.md"), []byte("hello"), 0o644))
		t.Setenv("AI_PROMPTS_EXECUTION_DIR", dir)

		fsys := InitAiPromptsFS(context.Background(), "", "1.0", "some-key")
		require.NotNil(t, fsys)
		assert.Equal(t, "hello", readFileFromFS(t, fsys, "system.md"))
	})

	t.Run("execution dir set but not readable yields nil", func(t *testing.T) {
		t.Setenv("AI_PROMPTS_EXECUTION_DIR", filepath.Join(t.TempDir(), "does-not-exist"))

		fsys := InitAiPromptsFS(context.Background(), "", "1.0", "some-key")
		assert.Nil(t, fsys)
	})
}

func Test_InitAiPromptsFS_Tier2_ServingDir(t *testing.T) {
	t.Run("resolves the right version directory locally, no network", func(t *testing.T) {
		server := failIfHitServer(t)
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "v1.0"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "v1.4"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "v2.1"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "v1.0", "system.md"), []byte("v1.0"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "v1.4", "system.md"), []byte("v1.4"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "v2.1", "system.md"), []byte("v1.4"), 0o644))

		fsys := InitAiPromptsFS(context.Background(), dir, "1.9", "some-key")
		require.NotNil(t, fsys)
		assert.Equal(t, "v1.4", readFileFromFS(t, fsys, "system.md"))
	})

	t.Run("empty serving dir content yields nil", func(t *testing.T) {
		dir := t.TempDir()
		fsys := InitAiPromptsFS(context.Background(), dir, "1.0", "some-key")
		assert.Nil(t, fsys)
	})

	t.Run("unreadable serving dir yields nil", func(t *testing.T) {
		fsys := InitAiPromptsFS(context.Background(), filepath.Join(t.TempDir(), "missing"), "1.0", "some-key")
		assert.Nil(t, fsys)
	})
}

func Test_InitAiPromptsFS_Tier3_Network(t *testing.T) {
	t.Run("downloads and unzips the prompts bundle", func(t *testing.T) {
		zipBytes := writeFullPromptsZip(t, map[string]string{models.AiAgentSystemPromptPath: "downloaded content"})
		var gotAuth, gotVersion string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotVersion = r.URL.Query().Get("prompts_version")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipBytes)
		}))
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		fsys := InitAiPromptsFS(context.Background(), "", "1.2.3", "the-license-key")
		require.NotNil(t, fsys)
		assert.Equal(t, "downloaded content", readFileFromFS(t, fsys, models.AiAgentSystemPromptPath))
		assert.Equal(t, "Bearer the-license-key", gotAuth)
		assert.Equal(t, "1.2", gotVersion)
	})

	t.Run("download missing an expected prompt file is rejected entirely", func(t *testing.T) {
		files := make(map[string]string, len(models.AiAgentExpectedFiles)-1)
		for _, path := range models.AiAgentExpectedFiles[1:] { // omit the first expected path
			files[path] = "content"
		}
		zipBytes := writePromptsZip(t, files)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipBytes)
		}))
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		fsys := InitAiPromptsFS(context.Background(), "", "1.0", "some-key")
		assert.Nil(t, fsys)
	})

	t.Run("non-200 response yields nil", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		fsys := InitAiPromptsFS(context.Background(), "", "1.0", "some-key")
		assert.Nil(t, fsys)
	})

	t.Run("5xx response is retried and succeeds once the server recovers", func(t *testing.T) {
		zipBytes := writeFullPromptsZip(t, map[string]string{models.AiAgentSystemPromptPath: "downloaded after retry"})
		var callCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipBytes)
		}))
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		fsys := InitAiPromptsFS(context.Background(), "", "1.0", "some-key")
		require.NotNil(t, fsys)
		assert.Equal(t, "downloaded after retry", readFileFromFS(t, fsys, models.AiAgentSystemPromptPath))
		assert.Equal(t, 3, callCount)
	})

	t.Run("non-retryable 4xx response is not retried", func(t *testing.T) {
		var callCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		fsys := InitAiPromptsFS(context.Background(), "", "1.0", "some-key")
		assert.Nil(t, fsys)
		assert.Equal(t, 1, callCount)
	})

	t.Run("truncated/corrupted zip body is retried and succeeds once the server recovers", func(t *testing.T) {
		zipBytes := writeFullPromptsZip(t, map[string]string{models.AiAgentSystemPromptPath: "downloaded after retry"})
		var callCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusOK)
			if callCount < 3 {
				_, _ = w.Write([]byte("not a valid zip archive"))
				return
			}
			_, _ = w.Write(zipBytes)
		}))
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		fsys := InitAiPromptsFS(context.Background(), "", "1.0", "some-key")
		require.NotNil(t, fsys)
		assert.Equal(t, "downloaded after retry", readFileFromFS(t, fsys, models.AiAgentSystemPromptPath))
		assert.Equal(t, 3, callCount)
	})

	t.Run("well-formed zip missing an expected file is not retried", func(t *testing.T) {
		files := make(map[string]string, len(models.AiAgentExpectedFiles)-1)
		for _, path := range models.AiAgentExpectedFiles[1:] { // omit the first expected path
			files[path] = "content"
		}
		zipBytes := writePromptsZip(t, files)
		var callCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipBytes)
		}))
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		fsys := InitAiPromptsFS(context.Background(), "", "1.0", "some-key")
		assert.Nil(t, fsys)
		assert.Equal(t, 1, callCount)
	})

	t.Run("unparseable version skips the network entirely", func(t *testing.T) {
		server := failIfHitServer(t)
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		fsys := InitAiPromptsFS(context.Background(), "", "dev", "some-key")
		assert.Nil(t, fsys)
	})

	t.Run("empty license key skips the network entirely", func(t *testing.T) {
		server := failIfHitServer(t)
		defer server.Close()
		useAiPromptsServerURL(t, server.URL)

		fsys := InitAiPromptsFS(context.Background(), "", "1.0", "")
		assert.Nil(t, fsys)
	})
}

func Test_zipToPromptsFS(t *testing.T) {
	t.Run("all expected files present and readable succeeds", func(t *testing.T) {
		zipBytes := writeFullPromptsZip(t, map[string]string{models.AiAgentSystemPromptPath: "system content"})

		fsys, err := zipToPromptsFS(zipBytes)
		require.NoError(t, err)
		require.NotNil(t, fsys)
		assert.Equal(t, "system content", readFileFromFS(t, fsys, models.AiAgentSystemPromptPath))
	})

	t.Run("a single missing expected file is an unrecoverable error", func(t *testing.T) {
		files := make(map[string]string, len(models.AiAgentExpectedFiles)-1)
		for _, path := range models.AiAgentExpectedFiles[1:] {
			files[path] = "content"
		}
		zipBytes := writePromptsZip(t, files)

		fsys, err := zipToPromptsFS(zipBytes)
		assert.Nil(t, fsys)
		require.Error(t, err)
		assert.True(t, retry.IsRecoverable(err) == false, "missing expected file should be unrecoverable")
	})

	t.Run("extra unlisted files in the archive are ignored", func(t *testing.T) {
		files := map[string]string{"some/unrelated/asset.png": "binary-ish content"}
		for _, path := range models.AiAgentExpectedFiles {
			files[path] = "content"
		}
		zipBytes := writePromptsZip(t, files)

		fsys, err := zipToPromptsFS(zipBytes)
		require.NoError(t, err)
		require.NotNil(t, fsys)
		_, statErr := fs.Stat(fsys, "some/unrelated/asset.png")
		assert.Error(t, statErr)
	})

	t.Run("corrupted zip bytes fail closed with a recoverable error, not a panic", func(t *testing.T) {
		fsys, err := zipToPromptsFS([]byte("this is not a zip file"))
		assert.Nil(t, fsys)
		require.Error(t, err)
		assert.True(t, retry.IsRecoverable(err), "corrupted archive should be retried")
	})
}
