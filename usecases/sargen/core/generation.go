package core

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/invopop/jsonschema"

	"github.com/checkmarble/marble-backend/models"
)

// Task tiers (TaskSpec.Tier).
const (
	TierLight = "light"
	TierHeavy = "heavy"
)

// TaskKindReview is the task kind that never writes the document: its
// Findings replace the open review findings of the declaration.
const TaskKindReview = "review"

// TaskSpec is one unit of generation work planned by a Format.
type TaskSpec struct {
	Kind string `json:"kind"`
	// TargetPath is the part of the document the task is in charge of.
	TargetPath Path `json:"target_path"`
	// AllowedPaths restricts what the task may write; empty = [TargetPath].
	AllowedPaths []Path `json:"allowed_paths"`
	// Inputs are the selected objects the task may read.
	Inputs []ObjectRef `json:"inputs"`
	// PromptPath is the path of the prompt file in the prompts FS (never
	// committed to this repository).
	PromptPath string `json:"prompt_path"`
	// Tier is TierLight or TierHeavy (model choice).
	Tier      string   `json:"tier"`
	ToolNames []string `json:"tool_names"`
	// DependsOnKinds: the task is ready once every task of these kinds
	// planned in the same job is done.
	DependsOnKinds []string `json:"depends_on_kinds"`
	// OutputSchema nil = the standard fills schema.
	OutputSchema *jsonschema.Schema `json:"output_schema,omitempty"`
	Payload      map[string]any     `json:"payload"`
}

// PlanState is what Format.Plan sees; it is called again after each wave of
// finished tasks, until it returns no spec.
type PlanState struct {
	JobKind     string
	Declaration models.SarDeclaration
	Doc         *Document
	Selection   []models.SarSelectedObject
	// Done are the tasks of the job already done (any wave, any attempt).
	Done   []models.SarGenerationTask
	Issues []Issue
}

type QuestionOption struct {
	Label string `json:"label"`
	// Fills are ready-to-apply values (with their sources); empty when the
	// option is only a label.
	Fills []Fill `json:"fills"`
}

type QuestionDraft struct {
	Paths   []Path           `json:"paths"`
	Text    string           `json:"text"`
	Options []QuestionOption `json:"options"`
}

type ReviewFinding struct {
	Paths []Path `json:"paths"`
	// Severity is "important" or "mineur".
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// TaskOutput is the result of a task run. The orchestrator never trusts it:
// it stamps Origin and TaskID on every fill and restricts paths.
type TaskOutput struct {
	Fills     []Fill          `json:"fills"`
	Questions []QuestionDraft `json:"questions"`
	Findings  []ReviewFinding `json:"findings"`
	// Structured is a non-fills output (e.g. placement), applied by
	// Format.ApplyStructured.
	Structured json.RawMessage `json:"structured,omitempty"`
	Trace      []byte          `json:"trace,omitempty"`
}

// MemoryRuleView is a memory rule shown to tools (always empty in V1).
type MemoryRuleView struct {
	Id          uuid.UUID
	Instruction string
	TargetPaths []string
	SameVersion bool
}

// ToolContext is built by the orchestrator from the declaration (its
// organization), never from an LLM output.
type ToolContext struct {
	OrgId, DeclarationId, TaskId uuid.UUID
	Format                       Format
	// Doc is a read-only copy.
	Doc       *Document
	Selection []models.SarSelectedObject
	DataModel models.DataModel
	Rules     []MemoryRuleView
}

// Tool is one LLM tool. Call never returns a Go error: any error is a JSON
// text {"error": "..."} readable by the LLM.
type Tool struct {
	Name, Description string
	Params            jsonschema.Schema
	Call              func(ctx context.Context, args json.RawMessage) string
}

// ComputeResult is the output of Format.Compute: derived fields.
type ComputeResult struct {
	// Fills may use provisional keys to create a row.
	Fills       []Fill
	RemoveItems []Path
}
