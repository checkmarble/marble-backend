package engine

import (
	"io"
	"testing"

	"github.com/checkmarble/llmberjack"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// The real adapter needs a provider; what can be checked offline is the
// request built for the first turn of a run.
func TestBuildTurnRequest_FirstTurn(t *testing.T) {
	tc := agentToolContext(t)
	tools := (&agentRunner{}).tools(tc, agentSpec("fill", agentPersonPath))
	require.NotEmpty(t, tools)
	schema := FillsOutputSchema()
	req := AgentRequest{
		RunID: "run", Tier: core.TierLight, Instruction: "consigne", Prompt: "prompt",
		Tools: tools, OutputSchema: schema,
	}

	r, err := buildTurnRequest(&llmRun{}, req, "main", "gemini-test")
	require.NoError(t, err)

	require.NotNil(t, r.Model)
	assert.Equal(t, "gemini-test", *r.Model)
	require.NotNil(t, r.SchemaOverride, "the final answer schema is sent with the tools")
	assert.Equal(t, schema.Required, r.SchemaOverride.Required)
	assert.NotNil(t, r.ResponseSchema, "a typed request, else the provider ignores the override")
	require.Len(t, r.Tools, len(tools))
	for _, ct := range tools {
		lt, ok := r.Tools[ct.Name]
		require.True(t, ok, ct.Name)
		assert.Equal(t, ct.Params, lt.Parameters, "tool parameters are the core tool schema")
		res, err := lt.Call([]byte(`{}`))
		require.NoError(t, err, "a tool never returns a Go error")
		assert.Contains(t, res, "error")
	}

	var texts []string
	for _, m := range r.Messages {
		for _, p := range m.Parts {
			b, _ := io.ReadAll(p)
			texts = append(texts, string(b))
		}
	}
	assert.Equal(t, []string{"consigne", "prompt"}, texts)
	assert.Equal(t, llmberjack.ThinkingLevelLow, *r.ThinkingLevel)
}

func readParts(t *testing.T, m llmberjack.Message) string {
	t.Helper()
	var s string
	for _, p := range m.Parts {
		b, err := io.ReadAll(p)
		require.NoError(t, err)
		s += string(b)
	}
	return s
}

func turnTwoRun(thread *llmberjack.ThreadId, calls ...llmberjack.ResponseToolCall) *llmRun {
	return &llmRun{
		prev: &llmberjack.Response[map[string]any]{
			InnerResponse: llmberjack.InnerResponse{Candidates: []llmberjack.ResponseCandidate{{
				ToolCalls: calls, SelectCandidate: func() {},
			}}},
			ThreadId: thread,
		},
		consumed: 1,
	}
}

func turnTwoRequest(t *testing.T, history ...AgentMessage) AgentRequest {
	t.Helper()
	tc := agentToolContext(t)
	return AgentRequest{
		RunID: "run", Tier: core.TierLight, Instruction: "i", Prompt: "p",
		Tools: (&agentRunner{}).tools(tc, agentSpec("fill", agentPersonPath)), OutputSchema: FillsOutputSchema(),
		History: append([]AgentMessage{{Role: RoleAssistant}}, history...),
	}
}

func TestBuildTurnRequest_SameToolTwiceKeepsCallOrder(t *testing.T) {
	thread := &llmberjack.ThreadId{}
	st := turnTwoRun(thread,
		llmberjack.ResponseToolCall{Id: "1", Name: "read_object", Parameters: []byte(`{"table":"persons","object_id":"p1"}`)},
		llmberjack.ResponseToolCall{Id: "2", Name: "read_object", Parameters: []byte(`{"table":"persons","object_id":"p2"}`)},
		llmberjack.ResponseToolCall{Id: "3", Name: "field_spec", Parameters: []byte(`null`)},
	)
	c1, c2, c3 := ToolCall{ID: "1", Name: "read_object"}, ToolCall{ID: "2", Name: "read_object"}, ToolCall{ID: "3", Name: "field_spec"}
	req := turnTwoRequest(t,
		AgentMessage{Role: RoleTool, Text: "result-one", ToolCall: &c1},
		AgentMessage{Role: RoleTool, Text: "result-two", ToolCall: &c2},
		AgentMessage{Role: RoleTool, Text: "result-spec", ToolCall: &c3},
	)
	r, err := buildTurnRequest(st, req, "main", "m")
	require.NoError(t, err)
	assert.Same(t, thread, r.ThreadId)

	var got [][2]string
	for _, m := range r.Messages {
		require.Equal(t, llmberjack.RoleTool, m.Role)
		got = append(got, [2]string{m.Tool.Id, readParts(t, m)})
	}
	assert.Equal(t, [][2]string{{"1", "result-one"}, {"2", "result-two"}, {"3", "result-spec"}}, got)
}

func TestBuildTurnRequest_CorrectionFollowsAFinal(t *testing.T) {
	thread := &llmberjack.ThreadId{}
	st := turnTwoRun(thread) // the previous candidate was a final answer
	req := turnTwoRequest(t,
		AgentMessage{Role: RoleAssistant, Text: "{}"},
		AgentMessage{Role: RoleUser, Text: "corrige ceci"},
	)
	r, err := buildTurnRequest(st, req, "main", "m")
	require.NoError(t, err)
	require.Len(t, r.Messages, 1)
	assert.Equal(t, llmberjack.RoleUser, r.Messages[0].Role)
	assert.Equal(t, "corrige ceci", readParts(t, r.Messages[0]))
}

func TestBuildTurnRequest_UnknownToolGetsTheRunnersErrorResult(t *testing.T) {
	thread := &llmberjack.ThreadId{}
	st := turnTwoRun(thread, llmberjack.ResponseToolCall{Id: "9", Name: "ghost_tool", Parameters: []byte(`"not an object"`)})
	call := ToolCall{ID: "9", Name: "ghost_tool"}
	msg := toolError("unknown tool: ghost_tool")
	req := turnTwoRequest(t, AgentMessage{Role: RoleTool, Text: msg, ToolCall: &call})
	r, err := buildTurnRequest(st, req, "main", "m")
	require.NoError(t, err)
	// llmberjack would have recorded an error (and no tool message) without the stub.
	require.Len(t, r.Messages, 1)
	assert.Equal(t, msg, readParts(t, r.Messages[0]))
	assert.Equal(t, "ghost_tool", r.Messages[0].Tool.Name)
	assert.Contains(t, r.Tools, "ghost_tool")
}

func TestBuildTurnRequest_EmptyCandidatesContinueTheThread(t *testing.T) {
	thread := &llmberjack.ThreadId{}
	st := &llmRun{prev: &llmberjack.Response[map[string]any]{ThreadId: thread}, consumed: 1}
	req := turnTwoRequest(t, AgentMessage{Role: RoleUser, Text: "réponse vide : rends la réponse"})
	r, err := buildTurnRequest(st, req, "main", "m")
	require.NoError(t, err, "no candidate to select must not be an error")
	assert.Same(t, thread, r.ThreadId)
	require.Len(t, r.Messages, 1)
	assert.Equal(t, "réponse vide : rends la réponse", readParts(t, r.Messages[0]))
}

func TestIsNoContent(t *testing.T) {
	assert.True(t, isNoContent(errors.New("LLM provider generated no content")))
	assert.True(t, isNoContent(errors.Wrap(errors.New("LLM provider generated no content"), "x")))
	assert.False(t, isNoContent(errors.New("Error 429")))
}

func TestUsageFromResponse_CarriesCacheReads(t *testing.T) {
	u := usageFromResponse(llmberjack.ResponseUsage{InputTokens: 100, OutputTokens: 7, CacheReadTokens: 60})
	assert.Equal(t, AgentUsage{InputTokens: 100, OutputTokens: 7, CachedInputTokens: 60}, u)
}
