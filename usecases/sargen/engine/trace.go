package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
)

// Trace entry types (TraceEntry.Type). TaskOutput.Trace is the JSON array of
// the entries, stored in sar_generation_tasks.trace.
const (
	TraceHeader     = "header"      // first element: provider, model, prompt_path, prompt_sha256, total tokens
	TraceInstruct   = "instruction" // system instruction
	TracePrompt     = "prompt"
	TraceToolCall   = "tool_call"   // name, args (truncated to TraceResultMax chars)
	TraceToolResult = "tool_result" // name, result (truncated to TraceResultMax chars)
	TraceOutput     = "output"      // each Final, raw JSON
	TraceCheckError = "check_error" // each output-control error sent back (or kept)
)

// TraceResultMax is the maximum length of a tool result in the trace.
const TraceResultMax = 20000

// TraceEntry is one element of the trace array. Only the fields of its Type are
// set.
type TraceEntry struct {
	Type         string `json:"type"`
	Provider     string `json:"provider,omitempty"`
	Model        string `json:"model,omitempty"`
	PromptPath   string `json:"prompt_path,omitempty"`
	PromptSHA256 string `json:"prompt_sha256,omitempty"`
	Name         string `json:"name,omitempty"`
	Args         string `json:"args,omitempty"`
	Text         string `json:"text,omitempty"` // instruction, prompt, result, output, error
	InputTokens  int64  `json:"input_tokens,omitempty"`
	OutputTokens int64  `json:"output_tokens,omitempty"`
	// DurationMs is the duration of the LLM step (on the first entry the step
	// produced) or of the tool call (tool_result).
	DurationMs int64 `json:"duration_ms,omitempty"`
}

// clean strips NUL characters, which Postgres rejects in text and jsonb.
func clean(s string) string { return strings.ReplaceAll(s, "\x00", "") }

// cleanJSON strips NUL characters from every string and key of a JSON document
// (a NUL arrives as the escape \u0000, invisible to a plain byte search).
func cleanJSON(raw json.RawMessage) json.RawMessage {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return json.RawMessage(strings.ReplaceAll(clean(string(raw)), `\u0000`, ""))
	}
	b, err := json.Marshal(cleanValue(v))
	if err != nil {
		return raw
	}
	return b
}

func cleanValue(v any) any {
	switch x := v.(type) {
	case string:
		return clean(x)
	case []any:
		for i := range x {
			x[i] = cleanValue(x[i])
		}
		return x
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, c := range x {
			out[clean(k)] = cleanValue(c)
		}
		return out
	default:
		return v
	}
}

// truncateResult bounds a tool result in the trace.
func truncateResult(s string) string {
	if r := []rune(s); len(r) > TraceResultMax {
		return string(r[:TraceResultMax]) + "… [truncated]"
	}
	return s
}

// trace accumulates the entries of one run; the header (first element) is
// completed when the first LLM step reports its provider and model.
type trace struct {
	entries []TraceEntry
}

func newTrace(promptPath, promptText string) *trace {
	sum := sha256.Sum256([]byte(promptText))
	return &trace{entries: []TraceEntry{{
		Type: TraceHeader, PromptPath: clean(promptPath), PromptSHA256: hex.EncodeToString(sum[:]),
	}}}
}

func (t *trace) setProvider(provider, model string) {
	if t.entries[0].Provider == "" {
		t.entries[0].Provider, t.entries[0].Model = clean(provider), clean(model)
	}
}

func (t *trace) add(e TraceEntry) {
	e.Name, e.Args, e.Text = clean(e.Name), clean(e.Args), clean(e.Text)
	t.entries = append(t.entries, e)
}

// render encodes the entries; the header carries the total token usage of the
// run.
func (t *trace) render() []byte {
	entries := slices.Clone(t.entries)
	var in, out int64
	for _, e := range entries[1:] {
		in += e.InputTokens
		out += e.OutputTokens
	}
	entries[0].InputTokens, entries[0].OutputTokens = in, out
	b, err := json.Marshal(entries)
	if err != nil {
		return []byte("[]")
	}
	return b
}
