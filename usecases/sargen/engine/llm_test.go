package engine

import (
	"io"
	"testing"

	"github.com/checkmarble/llmberjack"
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
