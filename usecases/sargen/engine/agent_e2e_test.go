package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const e2eDatePath = "personnes.pp[pp-1].etatCivil.dateNaissance"

// e2eOrchestrator wires the real agent runner (scripted LLM) into the
// orchestrator on the in-memory fakes: one fill task on personnes.pp[pp-1],
// whose item source is persons/p1.
func e2eOrchestrator(t *testing.T, llm LLM) (*Orchestrator, *fakes) {
	t.Helper()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, func(d *core.Document) {
		p := d.AddItem("personnes.pp", "pp")
		d.ItemSources[p] = []core.ObjectRef{{Table: "persons", ObjectID: "p1"}}
	})
	f.Repo.selection = []models.SarSelectedObject{
		agentSelection("persons", "p1", map[string]any{"object_id": "p1", "last_name": "Dupont", "birth_date": "1980-04-01"}),
	}
	spec := Spec("fill", tgtPP1)
	spec.Inputs = []core.ObjectRef{{Table: "persons", ObjectID: "p1"}}
	spec.ToolNames = []string{"read_object", "field_spec", "validate"}
	f.Format.Waves = [][]core.TaskSpec{{spec}}
	o.runner = NewAgentRunner(llm, fakeAgentPrompts{}, AgentLimits{})
	return o, f
}

func TestRunJob_AgentRunnerEndToEnd(t *testing.T) {
	ctx := context.Background()
	parallel := ScriptedStep{Step: AgentStep{ToolCalls: []ToolCall{
		{ID: "c1", Name: "read_object", Args: []byte(agReadP1Args)},
		{ID: "c2", Name: "field_spec", Args: []byte(`{"path":"` + pathNomPP1 + `"}`)},
	}, Usage: AgentUsage{InputTokens: 100, OutputTokens: 10}}}
	bad := agFinal(agFill(pathNomPP1, "Dupont", agSrc("last_name", "Durand"))) // raw_value is not what the object holds
	good := `{"fills":[` +
		agFill(pathNomPP1, "Dupont", agSrc("last_name", "Dupont")) + `,` +
		agFill(pathImplPP1, implicationValue, "null", `"justification":"le dossier le dit"`, `"confidence":"high"`) +
		`],"questions":[{"paths":["` + e2eDatePath + `"],"text":"Quelle est la date de naissance ?","options":[]}]}`
	llm := newScriptedLLM(parallel, FinalStep(bad), FinalStep(good))
	o, f := e2eOrchestrator(t, llm)

	require.NoError(t, o.RunJob(ctx, f.JobId))
	require.Equal(t, 3, llm.Calls())

	// The two parallel calls were both answered, in order, before the next turn.
	hist := llm.Requests[1].History
	require.Len(t, hist, 3)
	assert.Equal(t, RoleAssistant, hist[0].Role)
	require.Len(t, hist[0].ToolCalls, 2)
	assert.Equal(t, "read_object", hist[1].ToolCall.Name)
	assert.Contains(t, hist[1].Text, "Dupont")
	assert.Equal(t, "field_spec", hist[2].ToolCall.Name)
	// The bad raw_value was sent back, then corrected.
	assert.Contains(t, agLastIsCorrection(t, llm.Requests[2]), "raw_value does not match")

	task := f.Repo.Task(t, "fill", tgtPP1)
	require.Equal(t, models.SarJobStatusDone, task.Status)

	nom, ok := valueOf(t, f, pathNomPP1)
	require.True(t, ok)
	assert.Equal(t, "Dupont", nom.Value)
	assert.Equal(t, core.OriginLLM, nom.Origin)
	require.NotNil(t, nom.TaskID)
	assert.Equal(t, task.Id, *nom.TaskID)
	require.Len(t, nom.Sources, 1)
	assert.Equal(t, core.SourceRef{Object: core.ObjectRef{Table: "persons", ObjectID: "p1"}, Field: "last_name", RawValue: "Dupont"}, nom.Sources[0])

	impl, ok := valueOf(t, f, pathImplPP1)
	require.True(t, ok)
	assert.Equal(t, implicationValue, impl.Value)
	assert.Equal(t, core.OriginLLM, impl.Origin)
	assert.Equal(t, core.ConfidenceHigh, impl.Confidence)

	qs := questionsOf(t, f, models.SarQuestionStatusOpen)
	require.Len(t, qs, 1)
	assert.Equal(t, []string{e2eDatePath}, qs[0].Paths)
	assert.Equal(t, "Quelle est la date de naissance ?", qs[0].Text)
	require.NotNil(t, qs[0].TaskId)
	assert.Equal(t, task.Id, *qs[0].TaskId)

	// The trace is stored on the task.
	var trace []TraceEntry
	require.NoError(t, json.Unmarshal(task.Trace, &trace), string(task.Trace))
	assert.Equal(t, TraceHeader, trace[0].Type)
	assert.Equal(t, "fake-model", trace[0].Model)
	assert.EqualValues(t, 100, trace[0].InputTokens, "the header totals the usage")
	assert.True(t, agHasType(trace, TraceToolCall))
	assert.True(t, agHasType(trace, TraceCheckError), "the correction is traced")
	assert.True(t, agHasType(trace, TraceOutput))
}

func TestRunJob_AgentRunnerProviderErrorKeepsTraceOnFailedTask(t *testing.T) {
	ctx := context.Background()
	llm := newScriptedLLM(ToolCallStep("read_object", agReadP1Args), ErrStep(errors.New("fournisseur en panne")))
	o, f := e2eOrchestrator(t, llm)

	err := o.RunJob(ctx, f.JobId)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fournisseur en panne")

	task := f.Repo.Task(t, "fill", tgtPP1)
	require.Equal(t, models.SarJobStatusFailed, task.Status)
	var trace []TraceEntry
	require.NoError(t, json.Unmarshal(task.Trace, &trace), string(task.Trace))
	assert.Equal(t, "fake", trace[0].Provider)
	assert.True(t, agHasType(trace, TraceToolResult))
	assert.True(t, agHasType(trace, TraceCheckError), "the provider error is traced")
	_, written := valueOf(t, f, pathNomPP1)
	assert.False(t, written)
}
