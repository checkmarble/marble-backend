package tracfin

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAITraceOptInAndMalformedRaw(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TRACFIN_AI_DEBUG_DIR", dir)
	t.Setenv("TRACFIN_AI_DEBUG", "false")
	trace := NewAITrace("run")
	trace.Record("response", "persons", 1, "{malformed")
	trace.Close()
	entries, _ := os.ReadDir(dir)
	require.Empty(t, entries)
	t.Setenv("TRACFIN_AI_DEBUG", "true")
	trace = NewAITrace("run")
	trace.Record("response", "persons", 1, "{malformed")
	trace.Record("diagnostic", "persons", 1, "JSON_INVALID")
	trace.Close()
	require.NoError(t, trace.Err)
	raw, err := os.ReadFile(filepath.Join(trace.Path, "trace.jsonl"))
	require.NoError(t, err)
	require.Less(t, strings.Index(string(raw), "{malformed"), strings.Index(string(raw), "JSON_INVALID"))
	info, _ := os.Stat(trace.Path)
	require.Equal(t, os.FileMode(0700), info.Mode().Perm())
	info, _ = os.Stat(filepath.Join(trace.Path, "trace.jsonl"))
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
