package models

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

// AI metadata never enters the TRACFIN XML. It survives ordinary draft edits.
type TracfinAIEvidence struct {
	SourceID string `json:"source_id" jsonschema_description:"Exact ID of an entry of context.sources, or analysis:<analysis ID> for the human analysis."`
	Locator  string `json:"locator" jsonschema_description:"Case-sensitive JSON Pointer inside that source content, starting with / (for example /object/Data/last_name); /text for the analysis. Never a bare key, a dotted path or a composed value."`
}
type TracfinAIAnalysis struct {
	ID                   string    `json:"id"`
	Text                 string    `json:"text"`
	ActorID              string    `json:"actor_id"`
	CreatedAt            time.Time `json:"created_at"`
	AcceptedSuggestionID string    `json:"accepted_suggestion_id,omitempty"`
}
type TracfinAIMetadata struct {
	Analyses          []TracfinAIAnalysis `json:"analyses"`
	CurrentAnalysisID string              `json:"current_analysis_id"`
	RunID             string              `json:"run_id,omitempty"`
	ReviewedRevision  int64               `json:"reviewed_revision,omitempty"`
	ReviewedBy        string              `json:"reviewed_by,omitempty"`
}
type TracfinAIAttempt struct {
	Number      int                 `json:"number"`
	StartedAt   time.Time           `json:"started_at"`
	Raw         string              `json:"raw,omitempty"`
	Diagnostics []TracfinDiagnostic `json:"diagnostics,omitempty"`
	Status      string              `json:"status"`
}
type TracfinAIStep struct {
	// Accepted batches are immutable proposals, not fragments of truncated JSON.
	Batches           []json.RawMessage   `json:"batches,omitempty"`
	BatchAttemptStart int                 `json:"batch_attempt_start,omitempty"`
	BatchSize         int                 `json:"batch_size,omitempty"`
	Section           string              `json:"section"`
	Status            string              `json:"status"`
	Attempts          []TracfinAIAttempt  `json:"attempts"`
	Candidate         json.RawMessage     `json:"candidate,omitempty"`
	Diagnostics       []TracfinDiagnostic `json:"diagnostics,omitempty"`
	// Mode "amend" corrects the accepted section in place with a minimal patch,
	// following Instructions (analyst remark or blocking review findings),
	// instead of regenerating it.
	Mode         string `json:"mode,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

// TracfinAIChange records an accepted in-place correction, shown to the review
// so it can decide whether other sections need a correction too.
type TracfinAIChange struct {
	Section      string          `json:"section"`
	Origin       string          `json:"origin"` // analyst or review
	Instructions string          `json:"instructions"`
	Patch        json.RawMessage `json:"patch"`
}

// TracfinAIRemark is an analyst correction request made on a run of the case.
// Every later run receives all of them, so a correction is never forgotten.
type TracfinAIRemark struct {
	RunID     string    `json:"run_id"`
	Section   string    `json:"section"`
	Text      string    `json:"text"`
	ActorID   string    `json:"actor_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TracfinAIScopeCheck is the verdict on an analyst remark before a correction:
// a remark that is not a correction of the declaration content is refused.
type TracfinAIScopeCheck struct {
	InScope  bool               `json:"in_scope"`
	Sections []string           `json:"sections,omitempty"`
	Reason   string             `json:"reason,omitempty"`
	Attempts []TracfinAIAttempt `json:"attempts,omitempty"`
	Done     bool               `json:"done"`
}

// TracfinAIRunSummary is one entry of a case's AI history: what was asked,
// what each section became and what the review said, without prompts or raw
// model output.
type TracfinAIRunSummary struct {
	ID              string                 `json:"id"`
	ParentRunID     string                 `json:"parent_run_id,omitempty"`
	Kind            string                 `json:"kind"` // generation, correction or regeneration
	Status          string                 `json:"status"`
	CreatedAt       time.Time              `json:"created_at"`
	StartedAt       *time.Time             `json:"started_at,omitempty"`
	ActorID         string                 `json:"actor_id"`
	Model           string                 `json:"model,omitempty"`
	Calls           int                    `json:"calls"`
	Feedback        string                 `json:"feedback,omitempty"`
	FeedbackSection string                 `json:"feedback_section,omitempty"`
	ReworkReason    string                 `json:"rework_reason,omitempty"`
	Refusal         string                 `json:"refusal,omitempty"` // why a remark was refused (status refused)
	Steps           []TracfinAIStepSummary `json:"steps"`
	Changes         []TracfinAIChange      `json:"changes"`
	Review          json.RawMessage        `json:"review,omitempty"`
}
type TracfinAIStepSummary struct {
	Section      string              `json:"section"`
	Status       string              `json:"status"`
	Mode         string              `json:"mode,omitempty"`
	Instructions string              `json:"instructions,omitempty"`
	Attempts     int                 `json:"attempts"`
	Diagnostics  []TracfinDiagnostic `json:"diagnostics,omitempty"`
}
type TracfinAIRun struct {
	Changes         []TracfinAIChange    `json:"changes,omitempty"`
	Remarks         []TracfinAIRemark    `json:"remarks,omitempty"`
	ScopeCheck      *TracfinAIScopeCheck `json:"scope_check,omitempty"`
	EntityPlan      json.RawMessage      `json:"entity_plan,omitempty"`
	PlanAttempts    []TracfinAIAttempt   `json:"plan_attempts,omitempty"`
	CallBudget      int                  `json:"call_budget,omitempty"`
	BatchingVersion int                  `json:"batching_version,omitempty"`
	ReworkReason    string               `json:"rework_reason,omitempty"`
	ParentRunID     string               `json:"parent_run_id,omitempty"`
	FromSection     string               `json:"from_section,omitempty"`
	FeedbackSection string               `json:"feedback_section,omitempty"` // section the remark targets, kept when a full regeneration replaces FromSection
	Feedback        string               `json:"feedback,omitempty"`
	ReviewAttempts  []TracfinAIAttempt   `json:"review_attempts,omitempty"`
	PreviousSteps   [][]TracfinAIStep    `json:"previous_steps,omitempty"`
	ID              string               `json:"id"`
	OrgID           string               `json:"org_id"`
	CaseID          string               `json:"case_id"`
	ActorID         string               `json:"actor_id"`
	Revision        int64                `json:"revision"`
	DraftRevision   int64                `json:"draft_revision"`
	Status          string               `json:"status"`
	Context         TracfinAIContext     `json:"context"`
	AnalysisID      string               `json:"analysis_id,omitempty"`
	Prompts         map[string]string    `json:"-"`
	PromptSnapshot  map[string]string    `json:"prompt_snapshot,omitempty"`
	PromptHash      string               `json:"prompt_hash,omitempty"`
	CatalogHash     string               `json:"catalog_hash"`
	Provider        string               `json:"provider,omitempty"`
	Model           string               `json:"model,omitempty"`
	Steps           []TracfinAIStep      `json:"steps"`
	Review          json.RawMessage      `json:"review,omitempty"`
	ReviewCycle     int                  `json:"review_cycle"`
	Calls           int                  `json:"calls"`
	StartedAt       *time.Time           `json:"started_at,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	Validation      *TracfinValidation   `json:"validation,omitempty"`
	TraceError      string               `json:"trace_error,omitempty"`
}
type TracfinAIInput struct {
	Section          string `json:"section,omitempty"`
	Remarks          string `json:"remarks,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	Analysis         string `json:"analysis,omitempty"`
	SuggestionID     string `json:"suggestion_id,omitempty"`
}
type TracfinAIArgs struct {
	OrgID   uuid.UUID `json:"org_id"`
	CaseID  string    `json:"case_id"`
	RunID   string    `json:"run_id"`
	ActorID string    `json:"actor_id"`
}

func (TracfinAIArgs) Kind() string { return "tracfin_ai" }
