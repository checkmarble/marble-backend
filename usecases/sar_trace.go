package usecases

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/utils"
)

// sarTrace writes a local JSONL trace of an agent run (prompts, raw model
// responses, validation) when TRACFIN_AI_DEBUG=true. Debug only: 0700/0600,
// never uploaded; write failures never change the business result.
func sarTrace(ctx context.Context, id string) (func(kind, step string, attempt int, data any), func()) {
	if os.Getenv("TRACFIN_AI_DEBUG") != "true" {
		return nil, func() {}
	}
	base := os.Getenv("TRACFIN_AI_DEBUG_DIR")
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "sar-"+id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, func() {}
	}
	f, err := os.OpenFile(filepath.Join(dir, "trace.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, func() {}
	}
	utils.LoggerFromContext(ctx).InfoContext(ctx, "SAR agent trace", "path", f.Name())
	var mu sync.Mutex
	seq := 0
	return func(kind, step string, attempt int, data any) {
			mu.Lock()
			defer mu.Unlock()
			seq++
			line, _ := json.Marshal(map[string]any{"seq": seq, "at": time.Now().UTC(), "kind": kind, "step": step, "attempt": attempt, "data": data})
			_, _ = f.Write(append(line, '\n'))
		}, func() {
			_ = f.Close()
		}
}
