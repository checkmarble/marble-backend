package engine

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/checkmarble/llmberjack"
	"github.com/cockroachdb/errors"
	"github.com/invopop/jsonschema"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/ai_agent"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// LLM runs one turn of an agent conversation. The runner owns the loop (limits,
// output control, corrections, trace); an LLM only turns an AgentRequest into
// the model's next move. A provider failure is returned as an error (rate
// limits must stay detectable with errors.Is(err, models.LLMRateLimitedError)).
type LLM interface {
	Step(ctx context.Context, req AgentRequest) (AgentStep, error)
}

// Message roles of AgentMessage.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// RunEnder is optionally implemented by an LLM that keeps state per RunID.
type RunEnder interface{ EndRun(runID string) }

// AgentMessage is one turn of the conversation. The runner passes the whole
// conversation so far in AgentRequest.History (everything after the prompt):
// assistant tool calls, tool results (RoleTool, one message per ToolCall, same
// ID/Name), the corrections sent back after an invalid Final (RoleUser), and
// the assistant's invalid finals (RoleAssistant, Text = the JSON).
type AgentMessage struct {
	Role      string
	Text      string
	ToolCalls []ToolCall // RoleAssistant: the calls the model made
	ToolCall  *ToolCall  // RoleTool: the call this Text answers
}

// ToolCall is one tool invocation requested by the model.
type ToolCall struct {
	ID   string // may be empty (Gemini)
	Name string
	Args json.RawMessage
}

// AgentRequest is one LLM turn.
//
// Tools and OutputSchema are sent together on every turn (spike: tools and
// OverrideResponseSchema in the same request work on Gemini); the model ends
// the run by answering with JSON matching OutputSchema instead of calling a
// tool.
type AgentRequest struct {
	// RunID identifies the run (one conversation); an LLM holding provider
	// state per conversation keys it by RunID and is told when the run ends
	// through RunEnder.
	RunID       string
	Tier        string // core.TierLight / core.TierHeavy
	Instruction string // system instruction
	Prompt      string // first user message (task prompt)
	Tools       []core.Tool
	// OutputSchema is the schema of AgentStep.Final.
	OutputSchema jsonschema.Schema
	History      []AgentMessage
}

// AgentStep is the model's reply: either ToolCalls (run them, send the results
// back through History) or Final (JSON matching AgentRequest.OutputSchema).
// Parallel tool calls need llmberjack#15 (not merged): until then an
// implementation may return at most one call per step. Provider and Model are
// reported by the LLM (used for the trace header); Usage feeds the trace.
type AgentStep struct {
	ToolCalls []ToolCall
	Final     json.RawMessage
	Provider  string
	Model     string
	Usage     AgentUsage
}

type AgentUsage struct {
	InputTokens, OutputTokens int64
}

// llmberjackLLM is the production LLM. It gets its client from
// AiAgentUsecase.GetClient, resolves the model with
// models.AiAgentModelConfig.GetModel(models.AiFeatureSarGeneration, tier),
// routes it with ai_agent.ProviderForModel, and runs every request through
// ai_agent.DoLLMRequest. Tools are llmberjack.NewTool[map[string]any] whose
// exported Parameters field is replaced by core.Tool.Params; their Go function
// only returns the result the runner already computed (the runner owns tool
// execution, limits and trace), and a tool error is always text. The final
// answer is OverrideResponseSchema(req.OutputSchema) in the same request as the
// tools (spike). The request is typed (a response schema must be set for the
// provider to apply the override). One llmberjack thread per RunID.
// Parallel tool calls: keep the current llmberjack version (#15 not merged), so
// a model answering with several calls in one turn may be refused by Vertex.
type llmberjackLLM struct {
	agent  *ai_agent.AiAgentUsecase
	config *models.AiAgentModelConfig

	mu   sync.Mutex
	runs map[string]*llmRun
}

// llmRun is the thread state of one conversation.
type llmRun struct {
	prev     *llmberjack.Response[map[string]any]
	consumed int // History messages already sent
}

// NewLlmberjackLLM builds the production LLM.
func NewLlmberjackLLM(agent *ai_agent.AiAgentUsecase, config *models.AiAgentModelConfig) LLM {
	return &llmberjackLLM{agent: agent, config: config, runs: map[string]*llmRun{}}
}

func (l *llmberjackLLM) run(id string) *llmRun {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.runs[id]
	if !ok {
		r = &llmRun{}
		l.runs[id] = r
	}
	return r
}

// EndRun releases the thread of a finished run.
func (l *llmberjackLLM) EndRun(runID string) {
	l.mu.Lock()
	r := l.runs[runID]
	delete(l.runs, runID)
	l.mu.Unlock()
	if r != nil && r.prev != nil && r.prev.ThreadId != nil {
		r.prev.ThreadId.Close()
	}
}

func (l *llmberjackLLM) Step(ctx context.Context, req AgentRequest) (AgentStep, error) {
	if l.agent == nil || l.config == nil {
		return AgentStep{}, errors.New("modèle de génération non configuré")
	}
	model := l.config.GetModel(models.AiFeatureSarGeneration, models.AiModelTier(req.Tier))
	if model == "" {
		return AgentStep{}, errors.Newf("aucun modèle configuré pour la génération (niveau %s)", req.Tier)
	}
	provider := ai_agent.ProviderForModel(model)
	client, err := l.agent.GetClient(ctx)
	if err != nil {
		return AgentStep{}, errors.Wrap(err, "client LLM indisponible")
	}

	st := l.run(req.RunID)
	r, err := buildTurnRequest(st, req, provider, model)
	if err != nil {
		return AgentStep{}, err
	}
	resp, err := ai_agent.DoLLMRequest(ctx, client, r)
	if err != nil {
		return AgentStep{}, errors.Wrap(err, "appel du modèle")
	}
	st.consumed = len(req.History)
	st.prev = resp

	step := AgentStep{
		Provider: provider, Model: model,
		Usage: AgentUsage{InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens},
	}
	if len(resp.Candidates) == 0 {
		return step, nil
	}
	c := resp.Candidates[0]
	for _, tc := range c.ToolCalls {
		step.ToolCalls = append(step.ToolCalls, ToolCall{ID: tc.Id, Name: tc.Name, Args: json.RawMessage(tc.Parameters)})
	}
	if len(step.ToolCalls) == 0 && strings.TrimSpace(c.Text) != "" {
		step.Final = json.RawMessage(c.Text)
	}
	return step, nil
}

// buildTurnRequest builds the llmberjack request of the next turn of a run. On
// the first turn it opens the thread (instruction, prompt, tools); afterwards it
// continues from the previous candidate: tool results of History (consumed
// excluded) answer its tool calls, a user message (correction) follows a final.
func buildTurnRequest(st *llmRun, req AgentRequest, provider, model string) (llmberjack.Request[map[string]any], error) {
	r := llmberjack.NewRequest[map[string]any]().
		WithProvider(provider).
		WithModel(model).
		WithSchemaDescription("sar_generation_output", "").
		OverrideResponseSchema(req.OutputSchema).
		WithThinkingLevel(thinkingLevel(req.Tier))

	results := map[string][]string{} // tool name -> results in call order
	var corrections []string
	if st.prev != nil && st.consumed <= len(req.History) {
		for _, m := range req.History[st.consumed:] {
			switch {
			case m.Role == RoleTool && m.ToolCall != nil:
				results[m.ToolCall.Name] = append(results[m.ToolCall.Name], m.Text)
			case m.Role == RoleUser:
				corrections = append(corrections, m.Text)
			}
		}
	}

	for _, ct := range req.Tools {
		name := ct.Name
		t := llmberjack.NewTool[map[string]any](name, ct.Description, llmberjack.Function(func(map[string]any) (string, error) {
			q := results[name]
			if len(q) == 0 {
				return `{"error":"résultat indisponible"}`, nil
			}
			results[name] = q[1:]
			return q[0], nil
		}))
		t.Parameters = ct.Params
		r = r.WithTools(t)
	}

	if st.prev == nil {
		return r.CreateThread().
			WithInstruction(req.Instruction).
			WithText(llmberjack.RoleUser, req.Prompt), nil
	}
	r = r.FromCandidate(st.prev, 0)
	if len(st.prev.Candidates) > 0 && len(st.prev.Candidates[0].ToolCalls) > 0 {
		r = r.WithToolExecution()
	}
	for _, c := range corrections {
		r = r.WithText(llmberjack.RoleUser, c)
	}
	return r, nil
}

func thinkingLevel(tier string) llmberjack.ThinkingLevel {
	if tier == core.TierHeavy {
		return llmberjack.ThinkingLevelMedium
	}
	return llmberjack.ThinkingLevelLow
}
