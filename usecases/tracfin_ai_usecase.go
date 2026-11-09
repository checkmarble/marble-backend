package usecases

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ai_agent"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
)

type tracfinAILLM interface {
	TracfinAIPrompts() (map[string]string, error)
	TracfinAIModel() (string, string, error)
	TracfinAICompleteWithModel(context.Context, string, string, jsonschema.Schema, string, string) (ai_agent.TracfinAIResponse, error)
}
type TracfinAIDependencies struct {
	CheckActor func(context.Context, string, string) error
	Repository repositories.TracfinAIRepository
	LLM        tracfinAILLM
	Enqueue    func(context.Context, repositories.Transaction, models.TracfinAIArgs) error
}

func (uc TracfinUsecase) aiReady(ctx context.Context, caseID string) error {
	if _, err := uc.draftActor(ctx); err != nil {
		return err
	}
	c, err := uc.authorize(ctx, caseID)
	if err != nil {
		return err
	}
	if c.Type != models.CaseTypeDecision {
		return errors.Wrap(models.UnprocessableEntityError, "TRACFIN_AI_CASE_TYPE_UNSUPPORTED")
	}
	if uc.ai == nil || uc.ai.Repository == nil {
		return errors.New("TRACFIN AI dependencies unavailable")
	}
	return nil
}
func (uc TracfinUsecase) GetAIRun(ctx context.Context, caseID, id string) (models.TracfinAIRun, error) {
	if err := uc.aiReady(ctx, caseID); err != nil {
		return models.TracfinAIRun{}, err
	}
	return uc.ai.Repository.GetTracfinAIRun(ctx, uc.executorFactory.NewExecutor(), uc.orgID.String(), caseID, id)
}
func (uc TracfinUsecase) PrepareAI(ctx context.Context, caseID string, expected int64) (models.TracfinAIRun, error) {
	if err := uc.aiReady(ctx, caseID); err != nil {
		return models.TracfinAIRun{}, err
	}
	d, err := uc.GetDraft(ctx, caseID)
	if err != nil {
		return models.TracfinAIRun{}, err
	}
	if expected < 1 || d.Revision != expected {
		return models.TracfinAIRun{}, models.ConflictError
	}
	contextData, err := uc.CollectAIContext(ctx, caseID)
	if err != nil {
		return models.TracfinAIRun{}, err
	}
	c, err := tracfin.LoadCatalog()
	if err != nil {
		return models.TracfinAIRun{}, err
	}
	actor, _ := uc.draftActor(ctx)
	r := models.TracfinAIRun{ID: uuid.NewString(), OrgID: uc.orgID.String(), CaseID: caseID, ActorID: string(actor.UserId), Revision: 1, DraftRevision: expected, Status: "prepared", Context: contextData, CatalogHash: c.Hash, CreatedAt: time.Now().UTC()}
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		latest, e := uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, r.OrgID, caseID)
		if e != nil {
			return e
		}
		if latest.Revision != expected {
			return models.ConflictError
		}
		return uc.ai.Repository.CreateTracfinAIRun(ctx, tx, r)
	})
	return r, err
}
func validateAIAnalysis(text string) error {
	if strings.TrimSpace(text) == "" || !utf8.ValidString(text) || utf8.RuneCountInString(text) > 23000 {
		return errors.Wrap(models.BadParameterError, "analysis must contain 1 to 23000 characters; it is never rewritten or truncated")
	}
	return tracfin.CheckXMLText(text, models.TracfinFieldRef{FieldID: "/ds/analyse"})
}
func putAIAnalysis(d *models.TracfinDraft, text, actor, suggestion string) {
	if d.AI == nil {
		d.AI = &models.TracfinAIMetadata{}
	}
	a := models.TracfinAIAnalysis{ID: uuid.NewString(), Text: text, ActorID: actor, CreatedAt: time.Now().UTC(), AcceptedSuggestionID: suggestion}
	d.AI.Analyses = append(d.AI.Analyses, a)
	d.AI.CurrentAnalysisID = a.ID
	d.AI.ReviewedRevision = 0
	if d.Document.Root.Fields == nil {
		d.Document.Root.Fields = map[string][]models.TracfinValue{}
	}
	d.Document.Root.Fields["/ds/analyse"] = []models.TracfinValue{{ID: uuid.NewString(), Kind: "text", Text: &text, Origin: &models.TracfinOrigin{Source: "manual", Resolution: "resolved", Explanation: "Exact analyst text, explicitly submitted by the authenticated user", ActorID: actor, AnalysisID: a.ID, Evidence: []models.TracfinAIEvidence{{SourceID: "analysis:" + a.ID, Locator: "text"}}}}}
}
func (uc TracfinUsecase) SubmitAIAnalysis(ctx context.Context, caseID, id string, input models.TracfinAIInput) (models.TracfinDraft, error) {
	r, err := uc.GetAIRun(ctx, caseID, id)
	if err != nil {
		return models.TracfinDraft{}, err
	}
	if input.SuggestionID != "" {
		var review tracfin.AIReview
		if json.Unmarshal(r.Review, &review) != nil {
			return models.TracfinDraft{}, models.BadParameterError
		}
		found := false
		for _, f := range review.Findings {
			if f.ID == input.SuggestionID && f.SuggestedAnalysis != "" {
				input.Analysis = f.SuggestedAnalysis
				found = true
			}
		}
		if !found {
			return models.TracfinDraft{}, models.BadParameterError
		}
	}
	if err = validateAIAnalysis(input.Analysis); err != nil {
		return models.TracfinDraft{}, err
	}
	actor, _ := uc.draftActor(ctx)
	var result models.TracfinDraft
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		d, e := uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, r.OrgID, caseID)
		if e != nil {
			return e
		}
		if input.ExpectedRevision < 1 || d.Revision != input.ExpectedRevision {
			return models.ConflictError
		}
		if e = uc.ai.Repository.InvalidateTracfinAIActive(ctx, tx, r.OrgID, caseID); e != nil {
			return e
		}
		putAIAnalysis(&d, input.Analysis, string(actor.UserId), input.SuggestionID)
		d.UpdatedBy = string(actor.UserId)
		if e = uc.draftDependencies.Repository.SaveTracfinDraft(ctx, tx, r.OrgID, caseID, d, d.Revision); e != nil {
			return e
		}
		d.Revision++
		result = d
		return nil
	})
	return result, err
}
func currentAIAnalysis(d models.TracfinDraft) (models.TracfinAIAnalysis, error) {
	if d.AI != nil {
		for _, a := range d.AI.Analyses {
			if a.ID == d.AI.CurrentAnalysisID {
				values := d.Document.Root.Fields["/ds/analyse"]
				if len(values) == 1 && values[0].Text != nil && *values[0].Text == a.Text {
					return a, validateAIAnalysis(a.Text)
				}
			}
		}
	}
	return models.TracfinAIAnalysis{}, errors.Wrap(models.BadParameterError, "submit mandatory human analysis before generation")
}
func (uc TracfinUsecase) StartAI(ctx context.Context, caseID, id string, expected int64) (models.TracfinAIRun, error) {
	r, err := uc.GetAIRun(ctx, caseID, id)
	if err != nil {
		return r, err
	}
	if r.Status == "queued" || r.Status == "running" {
		if r.DraftRevision == expected {
			return r, nil
		}
		return r, models.ConflictError
	}
	if r.Status != "prepared" {
		return r, errors.Wrap(models.ConflictError, "prepare a new execution for a changed analysis or context")
	}
	d, err := uc.GetDraft(ctx, caseID)
	if err != nil {
		return r, err
	}
	if expected < 1 || d.Revision != expected {
		return r, models.ConflictError
	}
	a, err := currentAIAnalysis(d)
	if err != nil {
		return r, err
	}
	if uc.ai.LLM == nil || uc.ai.Enqueue == nil {
		return r, errors.New("TRACFIN AI model or queue unavailable")
	}
	r.PromptSnapshot, err = uc.ai.LLM.TracfinAIPrompts()
	if err != nil {
		return r, err
	}
	if r.PromptSnapshot == nil {
		r.PromptSnapshot = map[string]string{}
	}
	// Freeze backend-owned protocol instructions alongside the external prompt bundle.
	r.PromptSnapshot["builtin/tracfin/entity_plan_v2"] = aiEntityPlanInstructions + "\nInput JSON:\n{{.Input}}"
	r.PromptSnapshot["builtin/tracfin/entity_v2"] = aiEntityInstructions
	r.PromptSnapshot["builtin/tracfin/entity_finalize_v2"] = aiEntityFinalizationInstructions
	r.PromptSnapshot[aiAmendPromptPath] = aiAmendInstructions
	r.PromptSnapshot[aiScopePromptPath] = aiScopeInstructions
	r.Provider, r.Model, err = uc.ai.LLM.TracfinAIModel()
	if err != nil {
		return r, err
	}
	for _, path := range models.AiAgentTracfinPromptPaths {
		if strings.TrimSpace(r.PromptSnapshot[path]) == "" {
			return r, errors.New("required TRACFIN prompt unavailable: " + path)
		}
	}
	// Check every template before enqueue; the snapshot is persisted and never reread by this run.
	for _, content := range r.PromptSnapshot {
		if _, err = ai_agent.RenderTracfinPrompt(content, map[string]any{"Input": "{}"}); err != nil {
			return r, err
		}
	}
	frozen, _ := json.Marshal(r.PromptSnapshot)
	hash := sha256.Sum256(frozen)
	r.PromptHash = hex.EncodeToString(hash[:])
	latest, err := uc.CollectAIContext(ctx, caseID)
	if err != nil {
		return r, err
	}
	r.Context = latest
	baseline, _ := json.Marshal(dto.AdaptTracfinDraft(d).Document)
	baselineHash := sha256.Sum256(baseline)
	r.Context.Sources = append(r.Context.Sources, models.TracfinAISource{ID: "draft:" + r.ID, Kind: "draft_baseline", Locator: "document", SHA256: hex.EncodeToString(baselineHash[:]), Content: baseline})
	r.AnalysisID = a.ID
	r.DraftRevision = d.Revision
	r.Status = "queued"
	r.Steps = nil
	r.BatchingVersion = 2
	r.EntityPlan = nil
	r.PlanAttempts = nil
	r.CallBudget = 0
	r.Calls = 0
	r.ReviewCycle = 0
	r.ScopeCheck = nil
	var parent models.TracfinAIRun
	if r.ParentRunID != "" {
		parent, err = uc.GetAIRun(ctx, caseID, r.ParentRunID)
		if err != nil {
			return r, err
		}
	}
	if r.ParentRunID != "" && (parent.AnalysisID != a.ID || parent.Context.CaseFingerprint != latest.CaseFingerprint || d.AI == nil || d.AI.RunID != parent.ID) {
		r.ReworkReason = "Earlier automatic sections invalidated: current analysis, source snapshot or draft lineage differs from the parent run; requested section was " + r.FromSection
		r.FromSection = tracfin.AISections[0]
	}
	beforeTarget := r.FromSection != ""
	for i, section := range tracfin.AISections {
		status := "pending"
		if section == r.FromSection {
			beforeTarget = false
		}
		if beforeTarget && i < len(parent.Steps) {
			status = parent.Steps[i].Status
			if status != "accepted" {
				status = "partial"
			}
		}
		r.Steps = append(r.Steps, models.TracfinAIStep{Section: section, Status: status})
	}
	if r.ParentRunID != "" && r.ReworkReason == "" && len(parent.Steps) == len(tracfin.AISections) {
		// Same analysis, sources and lineage: correct only the requested section in place.
		aiPrepareAmend(&r, parent)
	}
	// Every remark made on the case's runs keeps applying: a new run receives them
	// all, each citable as its own analyst source, so no correction is forgotten.
	remarks, err := uc.ai.Repository.ListTracfinAIRemarks(ctx, uc.executorFactory.NewExecutor(), r.OrgID, caseID)
	if err != nil {
		return r, err
	}
	r.Remarks = remarks
	for _, remark := range remarks {
		if remark.RunID == r.ID {
			continue
		}
		content, _ := json.Marshal(map[string]string{"text": remark.Text, "section": remark.Section, "actor_id": remark.ActorID, "created_at": remark.CreatedAt.Format(time.RFC3339)})
		hash := sha256.Sum256(content)
		r.Context.Sources = append(r.Context.Sources, models.TracfinAISource{ID: "feedback:" + remark.RunID, Kind: "analyst_feedback", Locator: "text", SHA256: hex.EncodeToString(hash[:]), Content: content})
	}
	if r.Feedback != "" {
		feedback, _ := json.Marshal(map[string]string{"text": r.Feedback, "actor_id": r.ActorID})
		hash := sha256.Sum256(feedback)
		r.Context.Sources = append(r.Context.Sources, models.TracfinAISource{ID: "feedback:" + r.ID, Kind: "analyst_feedback", Locator: "text", SHA256: hex.EncodeToString(hash[:]), Content: feedback})
	}
	actor, _ := uc.draftActor(ctx)
	r.ActorID = string(actor.UserId)
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		fresh, e := uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, r.OrgID, caseID)
		if e != nil {
			return e
		}
		if fresh.Revision != expected {
			return models.ConflictError
		}
		if e = uc.ai.Repository.SaveTracfinAIRun(ctx, tx, r, r.Revision); e != nil {
			return e
		}
		return uc.ai.Enqueue(ctx, tx, models.TracfinAIArgs{OrgID: uc.orgID, CaseID: caseID, RunID: id, ActorID: r.ActorID})
	})
	if err == nil {
		r.Revision++
	}
	return r, err
}

// checkpoint is a CAS on both run and draft; no database transaction spans a model call.
func (uc TracfinUsecase) checkpointAI(ctx context.Context, r *models.TracfinAIRun, apply *models.TracfinDraft) error {
	if uc.ai.CheckActor != nil {
		if err := uc.ai.CheckActor(ctx, r.CaseID, r.ActorID); err != nil {
			// A cancelled context (worker shutdown) or an unavailable database is
			// transient: the job is retried. Only an access denial is final.
			if !aiAccessDenied(err) {
				return err
			}
			r.Status = "authorization_failed"
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if saveErr := uc.ai.Repository.SaveTracfinAIRun(cleanup, uc.executorFactory.NewExecutor(), *r, r.Revision); saveErr != nil {
				return saveErr
			}
			r.Revision++
			return err
		}
	}
	if _, err := uc.authorize(ctx, r.CaseID); err != nil {
		return err
	}
	stale := false
	err := uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		d, e := uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, r.OrgID, r.CaseID)
		if e != nil {
			return e
		}
		if d.Revision != r.DraftRevision {
			r.Status = "stale"
			stale = true
			apply = nil
		}
		if apply != nil {
			apply.UpdatedBy = r.ActorID
			if apply.AI == nil {
				apply.AI = &models.TracfinAIMetadata{}
			}
			apply.AI.RunID = r.ID
			apply.AI.ReviewedRevision = 0
			if e = uc.draftDependencies.Repository.SaveTracfinDraft(ctx, tx, r.OrgID, r.CaseID, *apply, r.DraftRevision); e != nil {
				return e
			}
			r.DraftRevision++
		}
		return uc.ai.Repository.SaveTracfinAIRun(ctx, tx, *r, r.Revision)
	})
	if err == nil {
		r.Revision++
	}
	if err == nil && stale {
		return models.ConflictError
	}
	return err
}
func (uc TracfinUsecase) ResumeAI(ctx context.Context, caseID, id string, expected int64) (models.TracfinAIRun, error) {
	r, err := uc.GetAIRun(ctx, caseID, id)
	if err != nil {
		return r, err
	}
	if r.DraftRevision != expected || r.Status == "stale" || r.Status == "prepared" || r.Status == "completed" || r.Status == "needs_input" {
		return r, models.ConflictError
	}
	if uc.ai.Enqueue == nil {
		return r, errors.New("TRACFIN queue unavailable")
	}
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		d, e := uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, r.OrgID, caseID)
		if e != nil {
			return e
		}
		if d.Revision != expected {
			return models.ConflictError
		}
		r.Status = "queued"
		if e = uc.ai.Repository.SaveTracfinAIRun(ctx, tx, r, r.Revision); e != nil {
			return e
		}
		return uc.ai.Enqueue(ctx, tx, models.TracfinAIArgs{OrgID: uc.orgID, CaseID: caseID, RunID: id, ActorID: r.ActorID})
	})
	if err == nil {
		r.Revision++
	}
	return r, err
}
func (uc TracfinUsecase) ApproveAI(ctx context.Context, caseID, id string, expected int64) (models.TracfinDraft, error) {
	r, err := uc.GetAIRun(ctx, caseID, id)
	if err != nil {
		return models.TracfinDraft{}, err
	}
	if r.Status != "completed" || r.DraftRevision != expected || !aiValidationPassed(r.Validation) {
		return models.TracfinDraft{}, errors.Wrap(models.UnprocessableEntityError, "TRACFIN AI validation incomplete")
	}
	actor, _ := uc.draftActor(ctx)
	var result models.TracfinDraft
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		d, e := uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, r.OrgID, caseID)
		if e != nil {
			return e
		}
		if d.Revision != expected || d.AI == nil || d.AI.RunID != id {
			return models.ConflictError
		}
		d.AI.ReviewedRevision = expected + 1
		d.AI.ReviewedBy = string(actor.UserId)
		d.UpdatedBy = string(actor.UserId)
		if e = uc.draftDependencies.Repository.SaveTracfinDraft(ctx, tx, r.OrgID, caseID, d, expected); e != nil {
			return e
		}
		d.Revision++
		result = d
		return nil
	})
	return result, err
}
func aiValidationPassed(v *models.TracfinValidation) bool {
	if v == nil {
		return false
	}
	for _, required := range []string{"internal_structure", "references", "business", "xml_syntax"} {
		if v.Layers[required].Status != "passed" {
			return false
		}
	}
	official, ok := v.Layers["official_xsd"]
	if !ok || (official.Status != "passed" && official.Status != "unavailable") {
		return false
	}
	for name, layer := range v.Layers {
		if name != "official_xsd" && layer.Status != "passed" {
			return false
		}
	}
	for _, d := range v.Diagnostics {
		if d.Severity == "error" {
			return false
		}
	}
	return true
}
func aiSources(r models.TracfinAIRun) map[string]bool {
	s := map[string]bool{}
	for _, source := range r.Context.Sources {
		s[source.ID] = true
	}
	s["analysis:"+r.AnalysisID] = true
	return s
}
func aiError(code, message string) models.TracfinDiagnostic {
	return models.TracfinDiagnostic{Code: code, Message: message, Layer: "ai", Severity: "error"}
}
func aiRemoveGenerated(root *models.TracfinBlock, section string) {
	protectedBlocks := map[string]bool{}
	tracfin.WalkBlocks(*root, func(b models.TracfinBlock) {
		for _, values := range b.Fields {
			for _, v := range values {
				if v.Reference != nil && (v.Origin == nil || v.Origin.Source != "ai") {
					protectedBlocks[v.Reference.BlockID] = true
					protectedBlocks[v.Reference.PersonBlockID] = true
				}
			}
		}
	})
	var removable func(models.TracfinBlock) bool
	removable = func(b models.TracfinBlock) bool {
		if b.Origin == nil || b.Origin.Source != "ai" || protectedBlocks[b.ID] {
			return false
		}
		for _, values := range b.Fields {
			for _, v := range values {
				if v.Origin == nil || v.Origin.Source != "ai" {
					return false
				}
			}
		}
		for _, child := range b.Children {
			if !removable(child) {
				return false
			}
		}
		return true
	}
	var clean func(*models.TracfinBlock)
	clean = func(b *models.TracfinBlock) {
		manualOwners := map[string]bool{}
		for _, values := range b.Fields {
			for _, v := range values {
				if v.ParentValueID != "" && (v.Origin == nil || v.Origin.Source != "ai") {
					manualOwners[v.ParentValueID] = true
				}
			}
		}
		for id, values := range b.Fields {
			if !tracfin.AISectionOwns(section, id) {
				continue
			}
			kept := values[:0]
			for _, v := range values {
				if manualOwners[v.ID] && v.Origin != nil && v.Origin.Source == "ai" {
					origin := *v.Origin
					origin.Source = "manual"
					origin.Explanation += "; preserved because an analyst amended an attribute of this scalar"
					v.Origin = &origin
				}
				if v.Origin == nil || v.Origin.Source != "ai" {
					kept = append(kept, v)
				}
			}
			b.Fields[id] = kept
		}
		children := b.Children[:0]
		for _, child := range b.Children {
			if tracfin.AISectionOwns(section, child.CatalogID) && removable(child) {
				continue
			}
			clean(&child)
			children = append(children, child)
		}
		b.Children = children
	}
	// Decide removability before clearing scalar values; manual evidence is never inferred from emptiness.
	clean(root)
}

func (uc TracfinUsecase) callAI(ctx context.Context, r *models.TracfinAIRun, d models.TracfinDraft, section string, previous string, diags []models.TracfinDiagnostic, findings any, trace *tracfin.AITrace) (ai_agent.TracfinAIResponse, error) {
	a, err := currentAIAnalysis(d)
	if err != nil {
		return ai_agent.TracfinAIResponse{}, err
	}
	catalog, err := tracfin.LoadCatalog()
	if err != nil {
		return ai_agent.TracfinAIResponse{}, err
	}
	input := map[string]any{"section": section, "context": r.Context, "analysis": a, "document": dto.AdaptTracfinDraft(d).Document, "catalog": aiPromptCatalog(catalog, section), "previous": previous, "diagnostics": diags, "findings": findings}
	if len(r.Remarks) > 0 && section != "entity_plan" {
		// Analyst corrections from this and earlier runs, oldest first.
		input["analyst_remarks"] = r.Remarks
	}
	amendStep := aiAmendStep(r, section)
	if amendStep != nil {
		input["instructions"] = amendStep.Instructions
	}
	if section != "review" && section != "entity_plan" {
		instructions, e := ai_agent.RenderTracfinPrompt(r.PromptSnapshot["tracfin/sections/"+section+".md"], map[string]any{"Input": ""})
		if e != nil {
			return ai_agent.TracfinAIResponse{}, e
		}
		input["section_instructions"] = instructions
	}
	if section == "review" {
		input["catalog"] = aiReviewCatalog(catalog, d)
		input["validation"] = r.Validation
		if len(r.Changes) > 0 {
			// In-place corrections made in this run: the review decides whether
			// other sections must be corrected as a consequence.
			input["changes"] = r.Changes
		}
		if r.BatchingVersion == 2 {
			input["entity_plan"] = json.RawMessage(r.EntityPlan)
		}
	}
	batchStep := aiBatchStep(r, section)
	if amendStep != nil {
		batchStep = nil
	}
	if section == "entity_plan" {
		input["inventory"] = aiEntityInventory(*r)
		input["catalog"] = map[string]any{"persons": aiPromptCatalog(catalog, "persons"), "operations": aiPromptCatalog(catalog, "operations")}
	}
	if batchStep != nil && r.BatchingVersion == 2 {
		work, e := aiEntityWork(*r, *batchStep)
		if e != nil {
			return ai_agent.TracfinAIResponse{}, e
		}
		input["entity_work"] = work
		input["document"] = aiEntityDocumentIndex(d)
		plan, e := aiReadEntityPlan(*r)
		if e != nil {
			return ai_agent.TracfinAIResponse{}, e
		}
		index := make([]aiEntityBlockIndex, 0, len(plan.Tasks))
		for _, task := range plan.Tasks {
			index = append(index, aiEntityBlockIndex{Key: task.Key, CatalogID: task.CatalogID, InventoryID: task.InventoryID})
		}
		input["entity_plan"] = index
	} else if batchStep != nil {
		accepted, e := aiAcceptedBatches(*batchStep)
		if e != nil {
			return ai_agent.TracfinAIResponse{}, e
		}
		input["batch"] = map[string]any{"number": len(batchStep.Batches) + 1, "max_entities": aiBatchSize(*batchStep), "accepted_proposal": accepted, "instructions": aiBatchInstructions}
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return ai_agent.TracfinAIResponse{}, err
	}
	if len(raw) > 512<<10 {
		return ai_agent.TracfinAIResponse{}, errors.Wrap(errTracfinAIContextBudget, "input exceeds 512 KiB; no sources silently omitted")
	}
	path := "tracfin/sections/" + section + ".md"
	schema := jsonschema.Reflect(tracfin.AISectionProposal{})
	if batchStep != nil && r.BatchingVersion == 1 {
		schema = jsonschema.Reflect(tracfin.AISectionBatch{})
	}
	if section == "entity_plan" {
		path = "builtin/tracfin/entity_plan_v2"
		schema = jsonschema.Reflect(tracfin.AIEntityPlan{})
	} else if section == "review" {
		path = "tracfin/review.md"
		schema = jsonschema.Reflect(tracfin.AIReview{})
	} else if amendStep != nil {
		path = aiAmendPromptPath
		schema = jsonschema.Reflect(tracfin.AIAmendProposal{})
	} else if previous != "" {
		path = "tracfin/repair_section.md"
	}
	template := r.PromptSnapshot[path]
	if path == aiAmendPromptPath && template == "" {
		template = aiAmendInstructions // runs started before in-place corrections existed
	}
	prompt, err := ai_agent.RenderTracfinPrompt(template, map[string]any{"Input": string(raw)})
	if err != nil {
		return ai_agent.TracfinAIResponse{}, err
	}
	if len(template) == 0 || r.PromptSnapshot["tracfin/system.md"] == "" {
		return ai_agent.TracfinAIResponse{}, errors.New("frozen TRACFIN prompts missing")
	}
	logger := utils.LoggerFromContext(ctx).With("run_id", r.ID, "case_id", r.CaseID, "section", section, "call", r.Calls)
	if batchStep != nil {
		if r.BatchingVersion == 2 {
			work, _ := aiEntityWork(*r, *batchStep)
			key := ""
			if work.Task != nil {
				key = work.Task.Key
			}
			logger = logger.With("entity_key", key, "mode", work.Mode, "unit", work.Index, "units_total", work.Total)
		} else {
			logger = logger.With("batch", len(batchStep.Batches)+1, "max_entities", aiBatchSize(*batchStep))
		}
	}
	logger.InfoContext(ctx, "TRACFIN AI LLM request started", "provider", r.Provider, "model", r.Model, "prompt", path, "repair", previous != "")
	trace.Record("request", section, r.Calls, map[string]any{"system": r.PromptSnapshot["tracfin/system.md"], "prompt": prompt, "schema": schema, "provider": r.Provider, "model": r.Model})
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	started := time.Now()
	response, err := uc.ai.LLM.TracfinAICompleteWithModel(callCtx, r.PromptSnapshot["tracfin/system.md"], prompt, *schema, r.Provider, r.Model)
	trace.Record("response", section, r.Calls, map[string]any{"raw": response.Text, "finish_reason": response.FinishReason, "usage": response.Usage, "duration_ms": time.Since(started).Milliseconds(), "error_present": err != nil, "error": aiErrorText(err)})
	if err != nil {
		logger.ErrorContext(ctx, "TRACFIN AI LLM request failed", "duration_ms", time.Since(started).Milliseconds(), "error", err)
	} else {
		logger.InfoContext(ctx, "TRACFIN AI LLM response received", "duration_ms", time.Since(started).Milliseconds(), "finish_reason", response.FinishReason, "response_bytes", len(response.Text), "usage", response.Usage)
	}
	return response, err
}
func (uc TracfinUsecase) validateAIAll(ctx context.Context, r models.TracfinAIRun, d models.TracfinDraft) models.TracfinValidation {
	c, _ := tracfin.LoadCatalog()
	v := tracfin.ValidateDraft(d, c)
	v.Diagnostics = append(v.Diagnostics, tracfin.ValidateAIAmountTotals(d, c)...)
	if !r.Context.Complete {
		v.Layers["source_coverage"] = models.TracfinValidationLayer{Status: "failed", Reason: "Source collection or extraction incomplete"}
	}
	var xml bytes.Buffer
	contents, attachmentErr := uc.aiValidationAttachments(ctx, d)
	if attachmentErr != nil {
		v.Diagnostics = append(v.Diagnostics, aiError("ATTACHMENT_VALIDATION", attachmentErr.Error()))
		return v
	}
	_, err := tracfin.WriteXML(&xml, d, contents, c)
	if err != nil {
		v.Diagnostics = append(v.Diagnostics, aiError("XML_SERIALIZATION", err.Error()))
		return v
	}
	syntax := tracfin.ValidateXML(bytes.NewReader(xml.Bytes()), c)
	v.Layers["xml_syntax"] = syntax.Layers["xml_syntax"]
	layer, diags := tracfin.ValidateOfficialXSD(ctx, xml.Bytes())
	v.Layers["official_xsd"] = layer
	v.Diagnostics = append(v.Diagnostics, diags...)
	return v
}

// Supply the authoritative constraints and codes for this section without unrelated source manifests.
func aiPromptCatalog(c tracfin.Catalog, section string) any {
	fields := map[string]tracfin.AIFieldSpec{}
	codes := map[string][]tracfin.Code{}
	for id, f := range c.Fields {
		if f.Coverage == "excluded" {
			continue
		}
		// Calculated identifiers and the operations period/total are assigned by the
		// backend; showing them invites the model to fill them.
		if section != "review" && tracfin.AIBackendComputed(c, id) {
			continue
		}
		if section != "review" && !tracfin.AISectionOwns(section, id) {
			continue
		}
		// A compact contract: expected format, allowed values and help instead of raw XSD facets.
		fields[id] = c.AISpec(id)
		for _, catalog := range f.Catalogs {
			codes[catalog] = c.Codes[catalog]
		}
	}
	return map[string]any{"version": c.Version, "hash": c.Hash, "fields": fields, "codes": codes, "excluded_codes": c.ExcludedCodes}
}

func (uc TracfinUsecase) aiValidationAttachments(ctx context.Context, d models.TracfinDraft) ([]tracfin.AttachmentContent, error) {
	var contents []tracfin.AttachmentContent
	var loadErr error
	var total int64
	tracfin.WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if loadErr != nil || b.Attachment == nil {
			return
		}
		a := b.Attachment
		if !attachmentProof(a, d.CaseID) || uc.attachmentDependencies() != nil {
			loadErr = errors.New("Attachment proof unavailable")
			return
		}
		file, e := uc.caseFile(ctx, uc.executorFactory.NewExecutor(), d.CaseID, a.SourceFileID)
		if e != nil {
			loadErr = e
			return
		}
		if file.FileReference != a.BlobKey || file.BucketName != a.BucketName {
			loadErr = errors.New("Attachment changed")
			return
		}
		blob, e := uc.draftDependencies.Blobs.GetBlob(ctx, a.BucketName, a.BlobKey)
		if e != nil {
			loadErr = e
			return
		}
		if blob.ReadCloser == nil {
			loadErr = errors.New("Attachment reader unavailable")
			return
		}
		raw, e := io.ReadAll(io.LimitReader(tracfinContextReader{ctx, blob.ReadCloser}, tracfin.MaxAttachmentBytes-total+1))
		closeErr := blob.ReadCloser.Close()
		if e != nil || closeErr != nil {
			loadErr = errors.Join(e, closeErr)
			return
		}
		total += int64(len(raw))
		hash := sha256.Sum256(raw)
		if total > tracfin.MaxAttachmentBytes || int64(len(raw)) != a.Size || hex.EncodeToString(hash[:]) != a.SHA256 {
			loadErr = errors.New("Attachment hash or size changed")
			return
		}
		contents = append(contents, tracfin.AttachmentContent{BlockID: b.ID, Metadata: *a, Reader: bytes.NewReader(raw)})
	})
	return contents, loadErr
}

var errTracfinAIContextBudget = errors.New("TRACFIN_AI_CONTEXT_BUDGET")

func aiReviewCatalog(c tracfin.Catalog, d models.TracfinDraft) any {
	fields := map[string]tracfin.Field{}
	labels := map[string]map[string]string{}
	tracfin.WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		for id, values := range b.Fields {
			f := c.Fields[id]
			fields[id] = f
			for _, v := range values {
				if v.Text == nil {
					continue
				}
				for _, catalog := range f.Catalogs {
					for _, code := range c.Codes[catalog] {
						if code.Value == *v.Text {
							if labels[catalog] == nil {
								labels[catalog] = map[string]string{}
							}
							labels[catalog][code.Value] = code.Label
						}
					}
				}
			}
		}
	})
	return map[string]any{"version": c.Version, "hash": c.Hash, "retained_fields": fields, "retained_code_labels": labels}
}

// ReworkAI starts a new traceable execution for explicit analyst feedback. Earlier
// sections remain accepted; the target and its automatic dependents are recomputed.
func (uc TracfinUsecase) ReworkAI(ctx context.Context, caseID, id string, input models.TracfinAIInput) (models.TracfinAIRun, error) {
	previous, err := uc.GetAIRun(ctx, caseID, id)
	if err != nil {
		return models.TracfinAIRun{}, err
	}
	valid := false
	for _, section := range tracfin.AISections {
		if section == input.Section {
			valid = true
		}
	}
	if !valid || strings.TrimSpace(input.Remarks) == "" || utf8.RuneCountInString(input.Remarks) > 8000 {
		return models.TracfinAIRun{}, errors.Wrap(models.BadParameterError, "known section and 1 to 8000 characters of remarks required")
	}
	if previous.DraftRevision > input.ExpectedRevision {
		return models.TracfinAIRun{}, models.ConflictError
	}
	r, err := uc.PrepareAI(ctx, caseID, input.ExpectedRevision)
	if err != nil {
		return r, err
	}
	r.ParentRunID = id
	r.FromSection = input.Section
	r.FeedbackSection = input.Section
	r.Feedback = input.Remarks
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		d, e := uc.draftDependencies.Repository.GetTracfinDraft(ctx, tx, r.OrgID, caseID)
		if e != nil {
			return e
		}
		if d.Revision != input.ExpectedRevision {
			return models.ConflictError
		}
		if e = uc.ai.Repository.InvalidateTracfinAIActive(ctx, tx, r.OrgID, caseID); e != nil {
			return e
		}
		return uc.ai.Repository.SaveTracfinAIRun(ctx, tx, r, r.Revision)
	})
	if err != nil {
		return r, err
	}
	r.Revision++
	return uc.StartAI(ctx, caseID, r.ID, input.ExpectedRevision)
}

func aiAccessDenied(err error) bool {
	return errors.Is(err, models.NotFoundError) || errors.Is(err, models.ForbiddenError) || errors.Is(err, models.UnAuthorizedError)
}

// aiInterrupted reports a model call stopped by the worker itself (shutdown or
// job cancellation), not by the model provider.
func aiInterrupted(ctx context.Context, callErr error) bool {
	return callErr != nil && errors.Is(ctx.Err(), context.Canceled)
}

// releaseInterruptedAI persists a run whose last reserved attempt was released
// after an interruption, so the retried job redoes that call.
func (uc TracfinUsecase) releaseInterruptedAI(ctx context.Context, r *models.TracfinAIRun, callErr error) error {
	utils.LoggerFromContext(ctx).WarnContext(ctx, "TRACFIN AI model call interrupted by the worker; attempt released", "run_id", r.ID, "error", callErr)
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := uc.ai.Repository.SaveTracfinAIRun(cleanup, uc.executorFactory.NewExecutor(), *r, r.Revision); err != nil {
		return errors.Join(callErr, err)
	}
	r.Revision++
	return errors.Wrap(callErr, "TRACFIN AI generation interrupted")
}

// aiModelFailure names the cause of a failed model call for the diagnostics.
func aiModelFailure(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "Model request timed out after 2 minutes"
	case errors.Is(err, models.LLMRateLimitedError):
		return "Model provider rate limit or quota reached (429 / RESOURCE_EXHAUSTED); retry later"
	default:
		return "Model provider error: " + aiErrorText(err)
	}
}

func aiErrorText(err error) string {
	if err == nil {
		return ""
	}
	text := strings.Join(strings.Fields(err.Error()), " ")
	if r := []rune(text); len(r) > 300 {
		text = string(r[:300]) + "…"
	}
	return text
}
