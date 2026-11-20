package engine

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
)

// ---------- scripted LLM ----------

// ScriptedStep is one scripted LLM reply: a step or a provider error.
type ScriptedStep struct {
	Step AgentStep
	Err  error
}

// scriptedLLM replays one ScriptedStep per Step call and records the requests.
// Past the end of the script it fails the call (provider error) so that a
// runaway loop is visible. Use ToolCallStep / FinalStep / ErrStep to script.
type scriptedLLM struct {
	mu    sync.Mutex
	steps []ScriptedStep
	// Requests holds a copy of every AgentRequest received, in order (History
	// is copied: it grows between calls).
	Requests []AgentRequest
}

func newScriptedLLM(steps ...ScriptedStep) *scriptedLLM { return &scriptedLLM{steps: steps} }

func (l *scriptedLLM) Step(_ context.Context, req AgentRequest) (AgentStep, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	req.History = append([]AgentMessage(nil), req.History...)
	l.Requests = append(l.Requests, req)
	n := len(l.Requests) - 1
	if n >= len(l.steps) {
		return AgentStep{}, errors.Newf("scripted LLM: no step %d", n)
	}
	s := l.steps[n]
	if s.Step.Provider == "" {
		s.Step.Provider = "fake"
	}
	if s.Step.Model == "" {
		s.Step.Model = "fake-model"
	}
	return s.Step, s.Err
}

// Calls is the number of Step calls so far.
func (l *scriptedLLM) Calls() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.Requests)
}

// ToolCallStep scripts one tool call (argsJSON is the raw JSON arguments).
func ToolCallStep(name, argsJSON string) ScriptedStep {
	return ScriptedStep{Step: AgentStep{ToolCalls: []ToolCall{{ID: "call-" + name, Name: name, Args: json.RawMessage(argsJSON)}}}}
}

// FinalStep scripts a final answer (raw JSON).
func FinalStep(finalJSON string) ScriptedStep {
	return ScriptedStep{Step: AgentStep{Final: json.RawMessage(finalJSON)}}
}

// ErrStep scripts a provider error.
func ErrStep(err error) ScriptedStep { return ScriptedStep{Err: err} }

// ---------- prompts ----------

// fakeAgentPrompts is a PromptReader: Texts[path] (default "prompt of <path>").
type fakeAgentPrompts struct{ Texts map[string]string }

func (p fakeAgentPrompts) Exists(path string) bool { return true }

func (p fakeAgentPrompts) Read(path string) (string, error) {
	if t, ok := p.Texts[path]; ok {
		return t, nil
	}
	return "prompt of " + path, nil
}

// ---------- tool context ----------

const (
	agentPersonPath  = core.Path("personnes.pp[pp-1]")
	agentPerson2Path = core.Path("personnes.pp[pp-2]")
)

// agentSelection returns a selected object whose snapshot went through a JSON
// round-trip like the repository's (numbers become float64).
func agentSelection(table, id string, snapshot map[string]any) models.SarSelectedObject {
	b, err := json.Marshal(snapshot)
	if err != nil {
		panic(err)
	}
	var rt map[string]any
	if err := json.Unmarshal(b, &rt); err != nil {
		panic(err)
	}
	return models.SarSelectedObject{TableName: table, ObjectId: id, Snapshot: rt}
}

// agentToolContext builds a ToolContext on the real TRACFIN format with a
// document holding two persons personnes.pp[pp-1] (ItemSources: persons/p1) and
// personnes.pp[pp-2] (persons/p2), and the given selection. The data model is
// toolsDataModel() (tools_test.go): replace tc.DataModel for other links.
func agentToolContext(t *testing.T, selection ...models.SarSelectedObject) core.ToolContext {
	t.Helper()
	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)
	doc := core.NewDocument()
	p1 := doc.AddItem("personnes.pp", "pp")
	doc.ItemSources[p1] = []core.ObjectRef{{Table: "persons", ObjectID: "p1"}}
	p2 := doc.AddItem("personnes.pp", "pp")
	doc.ItemSources[p2] = []core.ObjectRef{{Table: "persons", ObjectID: "p2"}}
	require.Equal(t, agentPersonPath, p1)
	require.Equal(t, agentPerson2Path, p2)
	if len(selection) == 0 {
		selection = []models.SarSelectedObject{
			agentSelection("persons", "p1", map[string]any{"object_id": "p1", "last_name": "Dupont", "birth_date": "1980-04-01"}),
			agentSelection("persons", "p2", map[string]any{"object_id": "p2", "last_name": "Martin"}),
		}
	}
	return core.ToolContext{
		OrgId: uuid.New(), DeclarationId: uuid.New(), TaskId: uuid.New(),
		Format: f, Doc: doc, Selection: selection, DataModel: toolsDataModel(),
	}
}

// agentSpec is a fill task on the first person (light tier, common tools).
func agentSpec(kind string, target core.Path) core.TaskSpec {
	return core.TaskSpec{
		Kind: kind, TargetPath: target, PromptPath: "prompts/" + kind + ".md", Tier: core.TierLight,
		ToolNames: []string{"read_object", "follow_link", "field_spec", "search_code", "data_model", "validate"},
	}
}

func agentTask(spec core.TaskSpec) models.SarGenerationTask {
	return models.SarGenerationTask{Id: uuid.New(), Kind: spec.Kind, TargetPath: string(spec.TargetPath)}
}
