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

// TaskKindCompute is the engine-known task kind that derives backend-computed
// fields (Format.Compute). The orchestrator runs it without the TaskRunner. A
// Format may plan it (DependsOnKinds = the fill kinds it must follow) so that
// later tasks see the computed values; when the plan never contains one, the
// orchestrator computes once at the end of a generation job.
const TaskKindCompute = "compute"

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
	// FailJobOnLimit: a task ending on a limit (turns, tool calls, time) with
	// no usable output (no fill, finding nor structured output; questions do
	// not count) fails the job instead of being done empty. For a task the
	// whole generation depends on.
	FailJobOnLimit bool `json:"fail_job_on_limit,omitempty"`
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
	// RemoveItem is an item (within the paths of the question) that choosing the
	// option removes. Before the removal, the format may rewrite what refers to
	// the item (ItemRemovalRewriter): those fills are computed when the option is
	// chosen, never stored, because the document changes after the question is
	// asked. Optional: an option without it only writes its Fills.
	RemoveItem Path `json:"remove_item,omitempty"`
	// Clear are paths (within the paths of the question) whose value choosing the
	// option deletes, before its Fills are written (a free-text name replaced by a
	// link to a declared person, for example). Settings and computed fields are
	// never cleared. Optional.
	Clear []Path `json:"clear,omitempty"`
}

type QuestionDraft struct {
	Paths   []Path           `json:"paths"`
	Text    string           `json:"text"`
	Options []QuestionOption `json:"options"`
	// Blocking: the declaration cannot be exported until the question is
	// answered (or closed).
	Blocking bool `json:"blocking,omitempty"`
}

// ItemRemovalRewriter is an optional capability of a Format: when a question
// option removes an item (QuestionOption.RemoveItem), RewriteBeforeRemoval
// returns, from the document as it is when the option is chosen, what keeps the
// rest of the document consistent once the item is gone. The fills (a reference
// to the item replaced by free text, for example) are applied with the analyst
// origin, over any existing value; the Clear paths are deleted (no value is
// stored there, so nothing later is blocked from filling them). They may write
// outside the paths of the question; they never write a setting or a computed
// field.
type ItemRemovalRewriter interface {
	RewriteBeforeRemoval(doc *Document, item Path) RemovalRewrite
}

// RemovalRewrite is the result of ItemRemovalRewriter.
type RemovalRewrite struct {
	Fills []Fill
	Clear []Path
}

// DocumentQuestioner is an optional capability of a Format: questions the
// backend raises from the document alone, before the final review (a match to
// confirm, a choice the data do not make). The engine calls it at that point of
// a job, drops the drafts whose paths already have a question (open or closed)
// and stores the others. It never writes the document. Option fills are checked
// by the engine like the fills of a task.
type DocumentQuestioner interface {
	DocumentQuestions(doc *Document) []QuestionDraft
}

// CodeSourceMapper is an optional capability of a Format: for a code list, the
// semantic types of the data-model columns whose value can feed it, and how a
// raw value of such a column is read as a code. The engine uses it to look for
// a value in the selected data before offering the unknown code.
type CodeSourceMapper interface {
	// CodeListSemantics returns the semantic types that feed the code list.
	CodeListSemantics(codeList string) []models.FieldSemanticType
	// CodeFromValue reads the raw value of a column of the given semantic type as
	// a code of the list; ok is false when it maps to no code.
	CodeFromValue(codeList string, semantic models.FieldSemanticType, raw string) (code string, ok bool)
}

type ReviewFinding struct {
	Paths []Path `json:"paths"`
	// Severity is "important" or "mineur".
	Severity string `json:"severity"`
	Message  string `json:"message"`
	// Options are proposals the analyst chooses with one click (already checked
	// like the fills of a task); empty for a finding that only reports.
	Options []QuestionOption `json:"options,omitempty"`
	// ContestsQuestionID is the open question of the declaration this finding
	// replaces (its data answer it): the question is closed as replaced and the
	// finding keeps its severity when it is blocking.
	ContestsQuestionID *uuid.UUID `json:"contests_question_id,omitempty"`
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
	// Issues are the export validation issues of the document, and OpenQuestions
	// the questions still open: given to the review task, which must not repeat
	// them and may contest a question.
	Issues        []Issue
	OpenQuestions []QuestionView
	Rules         []MemoryRuleView
}

// QuestionView is an open question as the review task sees it.
type QuestionView struct {
	Id           uuid.UUID `json:"id"`
	Kind         string    `json:"kind"`
	Severity     string    `json:"severity,omitempty"`
	Paths        []Path    `json:"paths"`
	Text         string    `json:"text"`
	OptionLabels []string  `json:"option_labels"`
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
