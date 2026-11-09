package tracfin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// AITrace contains only explicitly selected business payloads, never transport headers or model thoughts.
type AITrace struct {
	mu    sync.Mutex
	file  *os.File
	seq   int
	runID string
	Path  string
	Err   error
}

func NewAITrace(runID string) *AITrace {
	t := &AITrace{runID: runID}
	enabled, _ := strconv.ParseBool(os.Getenv("TRACFIN_AI_DEBUG"))
	if !enabled {
		return t
	}
	dir, err := os.MkdirTemp(os.Getenv("TRACFIN_AI_DEBUG_DIR"), "tracfin-ai-*")
	if err != nil {
		t.Err = err
		return t
	}
	t.Path = dir
	t.file, t.Err = os.OpenFile(filepath.Join(dir, "trace.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	return t
}
func (t *AITrace) Record(kind, section string, attempt int, data any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.file == nil || t.Err != nil {
		return
	}
	t.seq++
	t.Err = json.NewEncoder(t.file).Encode(struct {
		Sequence int       `json:"sequence"`
		At       time.Time `json:"at"`
		RunID    string    `json:"run_id"`
		Kind     string    `json:"kind"`
		Section  string    `json:"section,omitempty"`
		Attempt  int       `json:"attempt,omitempty"`
		Data     any       `json:"data"`
	}{t.seq, time.Now().UTC(), t.runID, kind, section, attempt, data})
	if t.Err == nil {
		t.Err = t.file.Sync()
	}
}
func (t *AITrace) Close() {
	if t.file != nil {
		if err := t.file.Close(); t.Err == nil {
			t.Err = err
		}
	}
}
