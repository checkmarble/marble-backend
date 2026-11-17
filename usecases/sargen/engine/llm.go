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
// Several calls in one step are supported by the runner (results come back in
// call order, one RoleTool message per call); whether the provider accepts them
// depends on the llmberjack version in use. Provider and Model are reported by
// the LLM (used for the trace header); Usage feeds the trace. A step with
// neither ToolCalls nor Final is an empty answer (provider returned nothing or
// blocked the content): the runner corrects, then ends on a quality limit.
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
//
// Mistakes of the model never become technical errors: a call to an unknown
// tool is answered by the runner's error text (a stub tool carries it), an
// answer without candidate or a provider "no content" is an empty step.
// Configuration errors (no config, no model for the tier, no client) wrap
// ErrLLMUnavailable: retrying cannot help, the worker cancels the River job.
//
// Parked: a failure of the FIRST turn of a run leaks its llmberjack thread
// (the thread is only returned with a successful response, so EndRun cannot
// close it); the history entry of that thread lives until the process ends.
type llmberjackLLM struct {
	agent  *ai_agent.AiAgentUsecase
	config *models.AiAgentModelConfig

	mu   sync.Mutex
	runs map[string]*llmRun
}

// ErrLLMUnavailable marks a configuration error of the model (not configured,
// no model for the tier, no client): non-retryable, the job is failed and the
// River job cancelled.
var ErrLLMUnavailable = errors.New("modèle de génération indisponible")

func unavailable(err error) error { return errors.Mark(err, ErrLLMUnavailable) }

// llmRun is the thread state of one conversation.
type llmRun struct {
	// prev is the last response of the thread. Without candidates (blocked or
	// empty answer) it only carries the thread: the next turn continues in it.
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

func (l *llmberjackLLM) Step(ctx context.Context, req AgentRequest) (step AgentStep, err error) {
	if l.agent == nil || l.config == nil {
		return AgentStep{}, unavailable(errors.New("modèle de génération non configuré"))
	}
	model := l.config.GetModel(models.AiFeatureSarGeneration, models.AiModelTier(req.Tier))
	if model == "" {
		return AgentStep{}, unavailable(errors.Newf("aucun modèle configuré pour la génération (niveau %s)", req.Tier))
	}
	provider := ai_agent.ProviderForModel(model)
	client, err := l.agent.GetClient(ctx)
	if err != nil {
		return AgentStep{}, unavailable(errors.Wrap(err, "client LLM indisponible"))
	}

	st := l.run(req.RunID)
	r, err := buildTurnRequest(st, req, provider, model)
	if err != nil {
		return AgentStep{}, err
	}
	// A malformed provider response must not crash the task: it is an empty
	// answer like the provider's own "no content".
	var resp *llmberjack.Response[map[string]any]
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				err = errors.Newf("%s (%v)", noContentMessage, rec)
			}
		}()
		resp, err = ai_agent.DoLLMRequest(ctx, client, r)
	}()
	step = AgentStep{Provider: provider, Model: model}
	if err != nil {
		if !isNoContent(err) {
			// Provider and model still reach the trace header.
			return step, errors.Wrap(err, "appel du modèle")
		}
		// The input of this turn is in the thread's history but nothing
		// answered it: the next turn continues the thread (not a candidate).
		st.consumed = len(req.History)
		if st.prev != nil {
			st.prev = &llmberjack.Response[map[string]any]{ThreadId: st.prev.ThreadId}
		}
		return step, nil
	}
	st.consumed = len(req.History)
	st.prev = resp

	step.Usage = AgentUsage{InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens}
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

// noContentMessage is the llmberjack/Gemini error for a candidate without
// parts (blocked or malformed answer).
const noContentMessage = "LLM provider generated no content"

func isNoContent(err error) bool {
	return strings.Contains(err.Error(), noContentMessage)
}

// buildTurnRequest builds the llmberjack request of the next turn of a run. On
// the first turn it opens the thread (instruction, prompt, tools); afterwards it
// continues from the previous candidate: tool results of History (consumed
// excluded) answer its tool calls (in call order, per tool name), a user
// message (correction) follows. When the previous response has no candidate the
// thread is continued directly (InThread), with the user messages only.
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

	// The Go function of a tool only hands back the result the runner already
	// computed for that call (one per call, in order).
	withStub := func(r llmberjack.Request[map[string]any], name, description string, params jsonschema.Schema,
	) llmberjack.Request[map[string]any] {
		// json.RawMessage accepts whatever arguments the model sent.
		t := llmberjack.NewTool[json.RawMessage](name, description, llmberjack.Function(func(json.RawMessage) (string, error) {
			q := results[name]
			if len(q) == 0 {
				return `{"error":"résultat indisponible"}`, nil
			}
			results[name] = q[1:]
			return q[0], nil
		}))
		t.Parameters = params
		return r.WithTools(t)
	}
	declared := map[string]bool{}
	for _, ct := range req.Tools {
		declared[ct.Name] = true
		r = withStub(r, ct.Name, ct.Description, ct.Params)
	}

	if st.prev == nil {
		return r.CreateThread().
			WithInstruction(req.Instruction).
			WithText(llmberjack.RoleUser, req.Prompt), nil
	}
	if len(st.prev.Candidates) == 0 {
		r = r.InThread(st.prev.ThreadId)
	} else {
		// A call without arguments arrives as null (or nothing): llmberjack
		// panics decoding it into the tool's argument type.
		for i, call := range st.prev.Candidates[0].ToolCalls {
			if p := strings.TrimSpace(string(call.Parameters)); p == "" || p == "null" {
				st.prev.Candidates[0].ToolCalls[i].Parameters = []byte("{}")
			}
		}
		r = r.FromCandidate(st.prev, 0)
		if calls := st.prev.Candidates[0].ToolCalls; len(calls) > 0 {
			// A tool the model made up is not in req.Tools: a stub for that name
			// lets llmberjack attach the runner's error result instead of failing.
			for _, call := range calls {
				if !declared[call.Name] {
					declared[call.Name] = true
					r = withStub(r, call.Name, "outil inconnu", jsonschema.Schema{Type: "object"})
				}
			}
			r = r.WithToolExecution()
		}
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
