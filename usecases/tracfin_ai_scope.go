package usecases

import (
	"context"
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/ai_agent"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/invopop/jsonschema"
)

// aiScopePromptPath is the frozen backend-owned instruction that screens analyst remarks.
const aiScopePromptPath = "builtin/tracfin/scope_v1"

const aiScopeInstructions = `You screen correction requests on a draft French TRACFIN suspicious-activity declaration generated from a case. The analyst wrote "remark" and chose "requested_section". Decide whether this correction tool can perform it.
In scope: correcting, adding or removing content of the declaration sections from the case sources or from facts the analyst states, even when those facts are not in the sources (for example "la nationalité est française", "ajoute le relevé en pièce jointe", "retire la personne X", "le motif de soupçon est ...", "le numéro de passeport est ...").
Out of scope, refuse:
- anything that is not a change to the declaration content: questions, conversation, explanations, opinions, translations, unrelated tasks;
- actions outside the draft: sending or submitting the declaration, contacting someone, changing the case, its status, alerts, decisions, transactions or ingested data, users or organization settings;
- rewriting or editing the human analysis: the analyst edits it directly;
- organisme, declarant or contact fields: they come from the organization settings;
- fields the backend computes: identifiers, operation counts, operations total and period, attachment name and MIME type (they derive from the operations and the files);
- inventing or guessing data with no basis ("mets des valeurs plausibles", "complète au hasard"), or bypassing validation or the official format;
- attaching a file that is not in the case files.
When a genuine correction is meant, accept it even if it is imprecise. List in sections every section the remark concerns (attachments, persons, operations, declaration), which may differ from requested_section.
Return in_scope, sections and reason: one short French sentence addressed to the analyst. When refusing, say what cannot be done here and, if relevant, where to do it.
Input JSON:
{{.Input}}`

var aiSectionLabels = map[string]string{
	"attachments": "pièces jointes (documents du case joints à la déclaration, type, date, description)",
	"persons":     "personnes physiques et morales, identités, rôles, références financières",
	"operations":  "opérations financières déclarées, montants, dates, émetteurs et bénéficiaires",
	"declaration": "contexte de la déclaration : cadre légal, nature du soupçon, indicateurs",
}

// checkAIRemarkScope screens the run's analyst remark before any change. It
// reports true when the remark is refused (the run is then final). A technical
// failure of the screening lets the correction proceed: it never blocks the analyst.
func (uc TracfinUsecase) checkAIRemarkScope(ctx context.Context, r *models.TracfinAIRun, trace *tracfin.AITrace) (bool, error) {
	logger := utils.LoggerFromContext(ctx).With("run_id", r.ID, "case_id", r.CaseID)
	if r.ScopeCheck == nil {
		r.ScopeCheck = &models.TracfinAIScopeCheck{}
	}
	input, _ := json.Marshal(map[string]any{"remark": r.Feedback, "requested_section": r.FeedbackSection, "sections": aiSectionLabels})
	template := r.PromptSnapshot[aiScopePromptPath]
	if template == "" {
		template = aiScopeInstructions // runs started before remark screening existed
	}
	prompt, err := ai_agent.RenderTracfinPrompt(template, map[string]any{"Input": string(input)})
	if err != nil {
		return false, err
	}
	schema := jsonschema.Reflect(tracfin.AIRemarkScope{})
	var verdict tracfin.AIRemarkScope
	accepted := false
	for len(r.ScopeCheck.Attempts) < 2 {
		r.ScopeCheck.Attempts = append(r.ScopeCheck.Attempts, models.TracfinAIAttempt{Number: len(r.ScopeCheck.Attempts) + 1, StartedAt: time.Now().UTC(), Status: "started"})
		if err = uc.checkpointAI(ctx, r, nil); err != nil {
			return false, err
		}
		trace.Record("request", "scope", len(r.ScopeCheck.Attempts), map[string]any{"prompt": prompt, "schema": schema, "model": r.Model})
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		started := time.Now()
		response, callErr := uc.ai.LLM.TracfinAICompleteWithModel(callCtx, r.PromptSnapshot["tracfin/system.md"], prompt, *schema, r.Provider, r.Model)
		cancel()
		trace.Record("response", "scope", len(r.ScopeCheck.Attempts), map[string]any{"raw": response.Text, "duration_ms": time.Since(started).Milliseconds(), "error": aiErrorText(callErr)})
		if aiInterrupted(ctx, callErr) {
			r.ScopeCheck.Attempts = r.ScopeCheck.Attempts[:len(r.ScopeCheck.Attempts)-1]
			return false, uc.releaseInterruptedAI(ctx, r, callErr)
		}
		at := &r.ScopeCheck.Attempts[len(r.ScopeCheck.Attempts)-1]
		at.Raw = response.Text
		at.Status = "rejected"
		switch {
		case callErr != nil:
			at.Diagnostics = []models.TracfinDiagnostic{aiError("MODEL_UNAVAILABLE", aiModelFailure(callErr))}
		default:
			verdict = tracfin.AIRemarkScope{}
			if e := tracfin.DecodeAIJSON([]byte(response.Text), &verdict); e != nil {
				at.Diagnostics = []models.TracfinDiagnostic{aiError("JSON_INVALID", e.Error())}
			} else {
				at.Status = "accepted"
				accepted = true
			}
		}
		if accepted {
			break
		}
	}
	r.ScopeCheck.Done = true
	if !accepted {
		logger.WarnContext(ctx, "TRACFIN AI remark screening unavailable; the correction proceeds")
		r.ScopeCheck.InScope = true
		return false, uc.checkpointAI(ctx, r, nil)
	}
	r.ScopeCheck.InScope = verdict.InScope
	r.ScopeCheck.Reason = verdict.Reason
	if !verdict.InScope {
		logger.InfoContext(ctx, "TRACFIN AI remark refused", "reason", verdict.Reason)
		r.Status = "refused"
		return true, uc.checkpointAI(ctx, r, nil)
	}
	// A correction also covers the other sections the remark concerns.
	for _, section := range verdict.Sections {
		for i := range r.Steps {
			if r.Steps[i].Section == section && r.Steps[i].Mode != "amend" && aiAmendStep(r, r.FeedbackSection) != nil {
				r.Steps[i] = models.TracfinAIStep{Section: section, Status: "pending", Mode: "amend", Instructions: r.Feedback}
				r.ScopeCheck.Sections = append(r.ScopeCheck.Sections, section)
			}
		}
	}
	return false, uc.checkpointAI(ctx, r, nil)
}
