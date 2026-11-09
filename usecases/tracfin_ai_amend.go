package usecases

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
)

// aiAmendPromptPath is the frozen backend-owned instruction for in-place corrections.
const aiAmendPromptPath = "builtin/tracfin/amend_v1"

const aiAmendInstructions = `Correct the accepted section named in "section" according to "instructions" (an analyst remark or blocking review findings). The section is already filled: "document" holds it with its block IDs. Return a MINIMAL patch, not the whole section:
- values: only the values to add or change. To change a value, set it on its existing block (block_key = the existing block ID): it replaces the generated value of that field. Do not restate unchanged values.
- blocks: only new blocks that the correction requires (new temporary keys).
- remove_blocks: IDs of generated blocks of this section to delete (for example a person who must not be declared).
- clear_values: generated values to delete without replacement ({block_key, field_id}).
- attachments, missing, conflicts: as for a section proposal; report in missing what the instructions ask but the sources cannot support.
Never undo what an earlier remark of "analyst_remarks" asked for, unless the current instructions explicitly change it. Keep existing block IDs so references from other sections stay valid. Manual values are protected: never target them. Never propose backend-computed fields (identifiers, operation counts, totals, period, attachment name or MIME type). Every value needs a short French explanation and precise evidence (source_id and JSON Pointer locator); the analyst remark is available as the analyst_feedback source. Follow "section_instructions" and the catalog contract for formats and allowed values. If "previous" and "diagnostics" are present, fix every diagnostic of that previous patch at once.
Input JSON:
{{.Input}}`

// aiAmendCallBudget bounds a targeted correction: the corrected section, the
// review, one correction of each other section requested by the review, and a
// second review, with three attempts each.
const aiAmendCallBudget = 3 + 3 + 3*(4-1) + 3

func aiAmendStep(r *models.TracfinAIRun, section string) *models.TracfinAIStep {
	for i := range r.Steps {
		if r.Steps[i].Section == section && r.Steps[i].Mode == "amend" {
			return &r.Steps[i]
		}
	}
	return nil
}

// aiNeedsEntityPlan reports whether a pending person or operation section must
// be generated from scratch, which is the only use of the entity work list.
func aiNeedsEntityPlan(r models.TracfinAIRun) bool {
	for _, s := range r.Steps {
		if s.Mode != "amend" && (s.Status == "pending" || s.Status == "running") && (s.Section == "persons" || s.Section == "operations") {
			return true
		}
	}
	return false
}

// aiPrepareAmend turns a rework into a targeted correction: the parent's accepted
// sections are kept as they are, and only the requested section is corrected in
// place, so unchanged content and block IDs survive.
func aiPrepareAmend(r *models.TracfinAIRun, parent models.TracfinAIRun) {
	raw, _ := json.Marshal(parent.Steps)
	var steps []models.TracfinAIStep
	_ = json.Unmarshal(raw, &steps)
	for i := range steps {
		if steps[i].Section == r.FromSection {
			steps[i] = models.TracfinAIStep{Section: steps[i].Section, Status: "pending", Mode: "amend", Instructions: r.Feedback}
		}
	}
	r.Steps = steps
	r.EntityPlan = parent.EntityPlan
	r.PlanAttempts = parent.PlanAttempts
	r.CallBudget = aiAmendCallBudget
}

// aiReviewInstructions turns blocking findings into correction instructions.
func aiReviewInstructions(findings []tracfin.AIReviewFinding) string {
	var lines []string
	for _, f := range findings {
		line := "- " + f.Message
		if f.FieldID != "" {
			line += " (" + f.FieldID + ")"
		}
		lines = append(lines, line)
	}
	return "Blocking review findings to fix:\n" + strings.Join(lines, "\n")
}

// aiScheduleReviewRepairs schedules an in-place correction of each section with
// blocking findings, instead of regenerating it and every later section.
func aiScheduleReviewRepairs(r *models.TracfinAIRun, repairs map[int][]tracfin.AIReviewFinding) {
	for i, findings := range repairs {
		if i < 0 || i >= len(r.Steps) {
			continue
		}
		r.Steps[i] = models.TracfinAIStep{Section: r.Steps[i].Section, Status: "pending", Mode: "amend", Instructions: aiReviewInstructions(findings)}
	}
	if r.BatchingVersion == 2 {
		// Each scheduled correction gets its own three attempts within the run budget.
		r.CallBudget = max(r.CallBudget, r.Calls+3*len(repairs)+3)
	}
}

func aiChangeOrigin(r models.TracfinAIRun) string {
	if r.ReviewCycle > 0 {
		return "review"
	}
	return "analyst"
}

// aiHistoryLimit bounds the case history returned to the frontend.
const aiHistoryLimit = 50

// ListAIHistory summarizes the case's AI runs, newest first.
func (uc TracfinUsecase) ListAIHistory(ctx context.Context, caseID string) ([]models.TracfinAIRunSummary, error) {
	if err := uc.aiReady(ctx, caseID); err != nil {
		return nil, err
	}
	runs, err := uc.ai.Repository.ListTracfinAIRuns(ctx, uc.executorFactory.NewExecutor(), uc.orgID.String(), caseID, aiHistoryLimit)
	if err != nil {
		return nil, err
	}
	out := make([]models.TracfinAIRunSummary, 0, len(runs))
	for _, r := range runs {
		out = append(out, aiRunSummary(r))
	}
	return out, nil
}

func aiRunSummary(r models.TracfinAIRun) models.TracfinAIRunSummary {
	// A correction edits sections in place; any other run with a parent regenerated them.
	kind := "generation"
	if r.ParentRunID != "" {
		kind = "regeneration"
		for _, step := range r.Steps {
			if step.Mode == "amend" && r.ReworkReason == "" {
				kind = "correction"
			}
		}
	}
	section := r.FeedbackSection
	if _, requested, ok := strings.Cut(r.ReworkReason, "requested section was "); section == "" && ok {
		section = requested // runs recorded before feedback_section existed
	}
	if section == "" && r.Feedback != "" {
		section = r.FromSection
	}
	s := models.TracfinAIRunSummary{ID: r.ID, ParentRunID: r.ParentRunID, Kind: kind, Status: r.Status, CreatedAt: r.CreatedAt, StartedAt: r.StartedAt, ActorID: r.ActorID, Model: r.Model, Calls: r.Calls, Feedback: r.Feedback, FeedbackSection: section, ReworkReason: r.ReworkReason, Steps: []models.TracfinAIStepSummary{}, Changes: r.Changes, Review: r.Review}
	if r.Status == "refused" && r.ScopeCheck != nil {
		s.Refusal = r.ScopeCheck.Reason
	}
	if s.Changes == nil {
		s.Changes = []models.TracfinAIChange{}
	}
	for _, step := range r.Steps {
		s.Steps = append(s.Steps, models.TracfinAIStepSummary{Section: step.Section, Status: step.Status, Mode: step.Mode, Instructions: step.Instructions, Attempts: len(step.Attempts), Diagnostics: step.Diagnostics})
	}
	return s
}
