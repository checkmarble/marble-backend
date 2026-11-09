package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

const aiMaxCalls = 26

func aiDeterministicEvidence(b *models.TracfinBlock, r models.TracfinAIRun) {
	if b.Origin == nil {
		b.Origin = &models.TracfinOrigin{Source: "calculated", Resolution: "resolved", Explanation: "Stable technical block identifier assigned by the backend", Evidence: []models.TracfinAIEvidence{{SourceID: "draft:" + r.ID, Locator: b.ID}}}
	}
	for field, values := range b.Fields {
		for i := range values {
			v := &values[i]
			if v.Origin == nil {
				v.Origin = &models.TracfinOrigin{Source: "manual", Resolution: "resolved"}
			}
			if v.Origin.Explanation == "" {
				v.Origin.Explanation = "Value retained from " + v.Origin.Source + " in the draft at the start of this execution"
				v.Origin.Evidence = []models.TracfinAIEvidence{{SourceID: "draft:" + r.ID, Locator: v.ID}}
			}
		}
		b.Fields[field] = values
	}
	for i := range b.Children {
		aiDeterministicEvidence(&b.Children[i], r)
	}
}

// applyAIAttachments returns errors that reject the proposal and warnings on the
// file-derived fields (name, MIME type) that keep the section partial.
func (uc TracfinUsecase) applyAIAttachments(ctx context.Context, r models.TracfinAIRun, d *models.TracfinDraft, p tracfin.AISectionProposal) []models.TracfinDiagnostic {
	var warnings []models.TracfinDiagnostic
	catalog, err := tracfin.LoadCatalog()
	if err != nil {
		return []models.TracfinDiagnostic{aiError("CATALOG_UNAVAILABLE", err.Error())}
	}
	aliases := map[string]string{}
	tracfin.WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		aliases[b.ID] = b.ID
		if b.Origin != nil && b.Origin.RunID == r.ID && b.Origin.SourceFingerprint != "" {
			aliases[b.Origin.SourceFingerprint] = b.ID
		}
	})

	for _, selection := range p.Attachments {
		fileID := strings.TrimPrefix(selection.SourceID, "file:")
		if fileID == selection.SourceID || !aiSources(r)[selection.SourceID] {
			return []models.TracfinDiagnostic{aiError("ATTACHMENT_SCOPE", "Attachment must identify a file in the immutable case inventory")}
		}
		target := aliases[selection.BlockKey]
		if target == "" {
			return []models.TracfinDiagnostic{aiError("ATTACHMENT_TARGET", "Cannot resolve selected attachment block")}
		}
		metadata, err := uc.selectTracfinAttachment(ctx, uc.executorFactory.NewExecutor(), r.CaseID, fileID)
		if err != nil {
			return []models.TracfinDiagnostic{aiError("ATTACHMENT_UNAVAILABLE", "Selected file is unavailable or changed")}
		}
		// The captured source hash describes the immutable bytes; compare the extracted inventory metadata too.
		for _, source := range r.Context.Sources {
			if source.ID == selection.SourceID {
				var file struct {
					SHA256 string `json:"sha256"`
				}
				_ = json.Unmarshal(source.Content, &file)
				if file.SHA256 == "" || file.SHA256 != metadata.SHA256 {
					return []models.TracfinDiagnostic{aiError("ATTACHMENT_CHANGED", "File changed since collection; prepare a new execution")}
				}
			}
		}
		setTracfinAttachment(&d.Document.Root, target, metadata)
		for _, source := range r.Context.Sources {
			if source.ID == selection.SourceID {
				var fieldDiags []models.TracfinDiagnostic
				*d, fieldDiags = aiSetAttachmentFileFields(*d, catalog, target, source, r.ID)
				warnings = append(warnings, fieldDiags...)
			}
		}
	}
	return warnings
}

// aiSetAttachmentFileFields fills an attachment's name and MIME type from the
// attached file: its case-file name and the type sniffed from its content. A
// type outside the allowed list is reported, never written.
func aiSetAttachmentFileFields(d models.TracfinDraft, c tracfin.Catalog, blockID string, source models.TracfinAISource, runID string) (models.TracfinDraft, []models.TracfinDiagnostic) {
	var file struct {
		Name string `json:"name"`
		Mime string `json:"mime"`
	}
	_ = json.Unmarshal(source.Content, &file)
	var diags []models.TracfinDiagnostic
	warn := func(code, field, message string) {
		diag := aiError(code, message)
		diag.Severity = "warning"
		diag.Field = &models.TracfinFieldRef{BlockID: blockID, FieldID: field}
		diags = append(diags, diag)
	}
	if name := strings.TrimSpace(file.Name); name != "" && utf8.RuneCountInString(name) <= 120 {
		d = tracfin.SetComputedValue(d, c, blockID, tracfin.AttachmentNameField, models.TracfinFieldInput{Kind: "text", Text: &name}, "Nom du fichier joint au case.", []models.TracfinAIEvidence{{SourceID: source.ID, Locator: "/name"}}, runID, false)
	} else {
		warn("ATTACHMENT_NAME_UNAVAILABLE", tracfin.AttachmentNameField, "The file name is empty or longer than 120 characters: set the attachment name manually")
	}
	var allowed []string
	for _, code := range c.Codes["liste_ref_typemime.csv"] {
		allowed = append(allowed, code.Value)
		if code.Value == file.Mime && file.Mime != "" {
			mime := file.Mime
			d = tracfin.SetComputedValue(d, c, blockID, tracfin.AttachmentMimeField, models.TracfinFieldInput{Kind: "text", Text: &mime}, "Type MIME détecté depuis le contenu du fichier.", []models.TracfinAIEvidence{{SourceID: source.ID, Locator: "/mime"}}, runID, false)
			return d, diags
		}
	}
	warn("ATTACHMENT_MIME_UNSUPPORTED", tracfin.AttachmentMimeField, fmt.Sprintf("The file content type %q is not in the allowed MIME list (%s): convert the file or set the type manually", file.Mime, strings.Join(allowed, ", ")))
	return d, diags
}

// GenerateAI resumes solely from persisted counters and accepted sections. A reserved failed
// or interrupted attempt still consumes its budget, so restarts cannot create an infinite loop.
func (uc TracfinUsecase) GenerateAI(ctx context.Context, caseID, id string) (resultErr error) {
	r, err := uc.GetAIRun(ctx, caseID, id)
	if err != nil {
		return err
	}
	if r.Status != "queued" && r.Status != "running" {
		return nil
	}
	logger := utils.LoggerFromContext(ctx).With("run_id", id, "case_id", caseID)
	started := time.Now()
	logger.InfoContext(ctx, "TRACFIN AI generation started", "status", r.Status, "calls", r.Calls, "model", r.Model)
	defer func() {
		if resultErr != nil {
			logger.ErrorContext(ctx, "TRACFIN AI generation interrupted", "status", r.Status, "calls", r.Calls, "duration_ms", time.Since(started).Milliseconds(), "error", resultErr)
			return
		}
		logger.InfoContext(ctx, "TRACFIN AI generation finished", "status", r.Status, "calls", r.Calls, "duration_ms", time.Since(started).Milliseconds())
	}()
	trace := tracfin.NewAITrace(id)
	defer func() {
		trace.Close()
		if trace.Err != nil {
			utils.LoggerFromContext(ctx).WarnContext(ctx, "TRACFIN AI local trace failed", "run_id", id, "error", trace.Err.Error())
		}
	}()
	if trace.Path != "" {
		utils.LoggerFromContext(ctx).InfoContext(ctx, "TRACFIN AI local trace", "run_id", id, "path", trace.Path)
	}
	trace.Record("resume", "", 0, r)
	if uc.ai.LLM == nil {
		return errors.New("TRACFIN model unavailable")
	}
	if r.StartedAt == nil && uc.aiCollector != nil {
		latest, e := uc.CollectAIContext(ctx, caseID)
		if e != nil {
			return e
		}
		if latest.CaseFingerprint != r.Context.CaseFingerprint {
			r.Status = "stale"
			return uc.checkpointAI(ctx, &r, nil)
		}
	}
	if r.StartedAt == nil {
		now := time.Now().UTC()
		r.StartedAt = &now
	}
	if time.Since(*r.StartedAt) > 20*time.Minute {
		r.Status = "exhausted"
		return uc.checkpointAI(ctx, &r, nil)
	}
	ctx, cancel := context.WithDeadline(ctx, r.StartedAt.Add(20*time.Minute))
	defer cancel()
	r.Status = "running"
	if err = uc.checkpointAI(ctx, &r, nil); err != nil {
		return err
	}
	catalog, err := tracfin.LoadCatalog()
	if err != nil {
		return err
	}
	if r.CatalogHash != catalog.Hash {
		r.Status = "stale"
		return uc.checkpointAI(ctx, &r, nil)
	}
	// A remark that is not a correction of the declaration is refused before any change.
	if r.Feedback != "" && (r.ScopeCheck == nil || !r.ScopeCheck.Done) {
		refused, e := uc.checkAIRemarkScope(ctx, &r, trace)
		if e != nil || refused {
			return e
		}
	}
	if d, e := uc.GetDraft(ctx, caseID); e != nil {
		return e
	} else if d.Revision != r.DraftRevision {
		r.Status = "stale"
		return uc.checkpointAI(ctx, &r, nil)
	} else if prefilled, applied, e := aiApplySettingsPrefill(d, catalog); e != nil {
		return e
	} else if applied > 0 {
		logger.InfoContext(ctx, "TRACFIN AI applied settings prefill", "values", applied)
		trace.Record("settings_prefill", "", 0, applied)
		if e = uc.checkpointAI(ctx, &r, &prefilled); e != nil {
			return e
		}
	}
	if r.BatchingVersion == 2 && aiNeedsEntityPlan(r) {
		ready, e := uc.prepareAIEntityPlan(ctx, &r, trace)
		if e != nil {
			return e
		}
		if !ready {
			return nil
		}
	}
	for {
		needsInput := false
		for i := range r.Steps {
			step := &r.Steps[i]
			if step.Status == "accepted" {
				continue
			}
			if step.Status == "partial" || step.Status == "exhausted" || step.Status == "blocked" {
				needsInput = true
				continue
			}
			// Operations rely on applied people/attachments: a partial section is applied, only with
			// reported missing data. Declaration may proceed with partial operations.
			if step.Section == "operations" && (!aiStepApplied(r.Steps[0]) || !aiStepApplied(r.Steps[1])) {
				logger.WarnContext(ctx, "TRACFIN AI section blocked by dependencies", "section", step.Section, "attachments_status", r.Steps[0].Status, "persons_status", r.Steps[1].Status)
				step.Status = "blocked"
				step.Diagnostics = []models.TracfinDiagnostic{aiError("DEPENDENCY_PENDING", "People and attachment sections must be applied (accepted or partial) before operations")}
				needsInput = true
				if err = uc.checkpointAI(ctx, &r, nil); err != nil {
					return err
				}
				continue
			}
			amending := step.Mode == "amend"
			batched := !amending && aiBatchStep(&r, step.Section) != nil
			for len(step.Attempts)-step.BatchAttemptStart < 3 && r.Calls < aiCallLimit(r) {
				d, e := uc.GetDraft(ctx, caseID)
				if e != nil {
					return e
				}
				if d.Revision != r.DraftRevision {
					r.Status = "stale"
					return uc.checkpointAI(ctx, &r, nil)
				}
				if _, e = currentAIAnalysis(d); e != nil {
					r.Status = "stale"
					return uc.checkpointAI(ctx, &r, nil)
				}
				// Clear only generated scalar values. Manual entries and manual clears remain protected.
				// An in-place correction starts from the accepted section instead.
				if !amending {
					aiRemoveGenerated(&d.Document.Root, step.Section)
				}
				attempt := models.TracfinAIAttempt{Number: len(step.Attempts) + 1, StartedAt: time.Now().UTC(), Status: "started"}
				previous := ""
				if len(step.Attempts) > step.BatchAttemptStart {
					previous = step.Attempts[len(step.Attempts)-1].Raw
				}
				step.Attempts = append(step.Attempts, attempt)
				r.Calls++
				if err = uc.checkpointAI(ctx, &r, nil); err != nil {
					return err
				}
				response, callErr := uc.callAI(ctx, &r, d, step.Section, previous, step.Diagnostics, json.RawMessage(r.Review), trace)
				if aiInterrupted(ctx, callErr) {
					step.Attempts = step.Attempts[:len(step.Attempts)-1]
					r.Calls--
					return uc.releaseInterruptedAI(ctx, &r, callErr)
				}
				if errors.Is(callErr, errTracfinAIContextBudget) {
					r.Calls--
					step.Status = "blocked"
					step.Diagnostics = []models.TracfinDiagnostic{aiError("CONTEXT_BUDGET", callErr.Error())}
					r.Status = "context_budget"
					return uc.checkpointAI(ctx, &r, nil)
				}
				at := &step.Attempts[len(step.Attempts)-1]
				at.Raw = response.Text
				at.Status = "rejected"
				step.Candidate = nil
				var p, batchProposal tracfin.AISectionProposal
				var amend tracfin.AIAmendProposal
				more := false
				var diags []models.TracfinDiagnostic
				if callErr != nil {
					diags = []models.TracfinDiagnostic{aiError("MODEL_UNAVAILABLE", aiModelFailure(callErr)+"; attempt consumed")}
				} else if response.FinishReason == "max_tokens" {
					if batched && r.BatchingVersion == 1 {
						step.BatchSize = max(1, aiBatchSize(*step)/2)
						logger.WarnContext(ctx, "TRACFIN AI truncated batch; retry with fewer entities", "section", step.Section, "batch", len(step.Batches)+1, "max_entities", step.BatchSize)
					}
					diags = []models.TracfinDiagnostic{aiError("OUTPUT_TRUNCATED", "Model response exceeded output budget")}
				} else {
					if amending {
						if e = tracfin.DecodeAIJSON([]byte(response.Text), &amend); e != nil {
							diags = []models.TracfinDiagnostic{aiError("JSON_INVALID", e.Error())}
						}
						p = amend.AISectionProposal
					} else if r.BatchingVersion == 2 && batched {
						p, batchProposal, more, diags = aiDecodeEntityBatch(response.Text, r, *step)
					} else if batched {
						p, batchProposal, more, diags = aiDecodeBatch(response.Text, *step)
					} else if e = tracfin.DecodeAIJSON([]byte(response.Text), &p); e != nil {
						diags = []models.TracfinDiagnostic{aiError("JSON_INVALID", e.Error())}
					}
				}
				if len(diags) == 0 {
					step.Candidate, _ = json.Marshal(p)
					// Evidence locators are checked in addition to source-ID membership.
					analysis, _ := currentAIAnalysis(d)
					diags = tracfin.ResolveAIAmounts(&p, r.Context.Sources)
					if len(diags) == 0 {
						diags = tracfin.ValidateAIProposalEvidence(p, r.Context.Sources, r.AnalysisID, analysis.Text)
					}
					for _, value := range p.Values {
						if value.Calculation != nil {
							target := catalog.Fields[value.FieldID].Currency
							if target == "" {
								for _, mapping := range d.Configuration.Mappings {
									if mapping.TargetField == value.FieldID && mapping.TargetCurrency != "" {
										if target != "" && target != mapping.TargetCurrency {
											target = "conflict"
											break
										}
										target = mapping.TargetCurrency
									}
								}
							}
							currency := value.Calculation.Currency
							if value.Calculation.Operation == "sum" && len(value.Calculation.Operands) > 0 {
								currency = value.Calculation.Operands[0].Currency
							}
							if target == "" || target != currency {
								diags = append(diags, aiError("AMOUNT_CURRENCY_UNPROVEN", "Target currency must be established by the catalog or configured mapping"))
							}
						}
					}
					if len(diags) == 0 {
						var proposed models.TracfinDraft
						if amending {
							amend.AISectionProposal = p
							proposed, diags = tracfin.ApplyAIAmend(d, step.Section, amend, aiSources(r), catalog, r.ID)
						} else {
							proposed, diags = tracfin.ApplyAISection(d, step.Section, p, aiSources(r), catalog, r.ID)
						}
						if step.Section == "operations" && !more && !aiHasErrors(diags) {
							diags = append(diags, tracfin.ValidateAIAmountTotals(proposed, catalog)...)
						}
						if !aiHasErrors(diags) {
							more := uc.applyAIAttachments(ctx, r, &proposed, p)
							diags = append(diags, more...)
							if !aiHasErrors(more) {
								d = proposed
							}
						}
					}
				}
				at.Diagnostics = diags
				step.Diagnostics = diags
				trace.Record("validation", step.Section, at.Number, diags)
				for _, diagnostic := range diags {
					logger.WarnContext(ctx, "TRACFIN AI validation diagnostic", "section", step.Section, "attempt", at.Number, "code", diagnostic.Code, "severity", diagnostic.Severity, "field", diagnostic.Field)
				}
				if aiHasErrors(diags) {
					logger.WarnContext(ctx, "TRACFIN AI attempt rejected; repair budget may remain", "section", step.Section, "attempt", at.Number-step.BatchAttemptStart, "section_attempt", at.Number, "max_attempts", 3)
				}
				if !aiHasErrors(diags) {
					at.Status = "accepted"
					if batched {
						// Store the original, unresolved proposal: monetary calculations are
						// revalidated from immutable evidence when assembling later batches.
						raw, _ := json.Marshal(batchProposal)
						step.Batches = append(step.Batches, raw)
						logger.InfoContext(ctx, "TRACFIN AI batch accepted", "section", step.Section, "batch", len(step.Batches), "has_more", more)
						trace.Record("batch_accepted", step.Section, at.Number, map[string]any{"batch": len(step.Batches), "has_more": more, "proposal": batchProposal})
						if more {
							step.BatchAttemptStart = len(step.Attempts)
							step.Diagnostics = nil
							if err = uc.checkpointAI(ctx, &r, nil); err != nil {
								return err
							}
							continue
						}
					}
					step.Status = "accepted"
					if len(p.Missing) > 0 || len(p.Conflicts) > 0 || len(diags) > 0 {
						step.Status = "partial"
						needsInput = true
					}
					logger.InfoContext(ctx, "TRACFIN AI section accepted", "section", step.Section, "status", step.Status, "attempt", at.Number, "missing_count", len(p.Missing), "conflict_count", len(p.Conflicts))
					step.Candidate, _ = json.Marshal(p)
					if amending {
						patch, _ := json.Marshal(amend)
						step.Candidate = patch
						r.Changes = append(r.Changes, models.TracfinAIChange{Section: step.Section, Origin: aiChangeOrigin(r), Instructions: step.Instructions, Patch: patch})
					}
					if step.Section == "operations" {
						// The period and total derive from the accepted lines: computed, never generated.
						var computed []models.TracfinDiagnostic
						d, computed = tracfin.ComputeAIOperationAggregates(d, catalog, r.Context.Sources, r.ID)
						if len(computed) > 0 {
							step.Diagnostics = append(step.Diagnostics, computed...)
							step.Status = "partial"
							needsInput = true
						}
						trace.Record("operations_aggregates", step.Section, at.Number, computed)
					}
					aiDeterministicEvidence(&d.Document.Root, r)
					if d.AI == nil || d.AI.CurrentAnalysisID != r.AnalysisID {
						r.Status = "stale"
						return uc.checkpointAI(ctx, &r, nil)
					}
					if err = uc.checkpointAI(ctx, &r, &d); err != nil {
						return err
					}
					trace.Record("applied", step.Section, at.Number, map[string]any{"revision": r.DraftRevision, "document": d.Document})
					break
				}
				if err = uc.checkpointAI(ctx, &r, nil); err != nil {
					return err
				}
			}
			if step.Status == "pending" || step.Status == "running" {
				logger.WarnContext(ctx, "TRACFIN AI section exhausted", "section", step.Section, "attempts", len(step.Attempts))
				step.Status = "exhausted"
				needsInput = true
				if err = uc.checkpointAI(ctx, &r, nil); err != nil {
					return err
				}
			}
		}
		d, e := uc.GetDraft(ctx, caseID)
		if e != nil {
			return e
		}
		validation := uc.validateAIAll(ctx, r, d)
		r.Validation = &validation
		trace.Record("global_validation", "", 0, validation)
		logger.InfoContext(ctx, "TRACFIN AI global validation finished", "passed", aiValidationPassed(r.Validation), "diagnostic_count", len(validation.Diagnostics))
		// The review can inspect a partial document; it never certifies backend validation.
		if r.Calls >= aiCallLimit(r) {
			r.Status = "exhausted"
			return uc.checkpointAI(ctx, &r, nil)
		}
		r.Calls++
		// The review gets the same bounded repair loop as sections: an invalid finding is
		// returned with its diagnostics instead of discarding the whole generated document.
		var review tracfin.AIReview
		var blocking bool
		var repairs map[int][]tracfin.AIReviewFinding
		previousReview := ""
		var reviewDiags []models.TracfinDiagnostic
		for try := 1; ; try++ {
			r.ReviewAttempts = append(r.ReviewAttempts, models.TracfinAIAttempt{Number: len(r.ReviewAttempts) + 1, StartedAt: time.Now().UTC(), Status: "started"})
			if err = uc.checkpointAI(ctx, &r, nil); err != nil {
				return err
			}
			response, callErr := uc.callAI(ctx, &r, d, "review", previousReview, reviewDiags, nil, trace)
			if aiInterrupted(ctx, callErr) {
				r.ReviewAttempts = r.ReviewAttempts[:len(r.ReviewAttempts)-1]
				r.Calls--
				return uc.releaseInterruptedAI(ctx, &r, callErr)
			}
			reviewAttempt := &r.ReviewAttempts[len(r.ReviewAttempts)-1]
			reviewAttempt.Raw = response.Text
			review = tracfin.AIReview{}
			switch {
			case callErr != nil:
				reviewDiags = []models.TracfinDiagnostic{aiError("MODEL_UNAVAILABLE", aiModelFailure(callErr))}
			case response.FinishReason == "max_tokens":
				reviewDiags = []models.TracfinDiagnostic{aiError("OUTPUT_TRUNCATED", "Review exceeded output budget; keep findings concise")}
			default:
				if e := tracfin.DecodeAIJSON([]byte(response.Text), &review); e != nil {
					reviewDiags = []models.TracfinDiagnostic{aiError("JSON_INVALID", e.Error())}
				} else {
					blocking, repairs, reviewDiags = validateAIReview(review, r, d, catalog)
				}
			}
			trace.Record("review_validation", "review", reviewAttempt.Number, reviewDiags)
			if len(reviewDiags) == 0 {
				reviewAttempt.Status = "accepted"
				break
			}
			reviewAttempt.Status = "rejected"
			reviewAttempt.Diagnostics = reviewDiags
			logger.WarnContext(ctx, "TRACFIN AI review rejected", "try", try, "diagnostic_count", len(reviewDiags))
			if try >= 3 || r.Calls >= aiCallLimit(r) {
				r.Status = "review_failed"
				r.Review, _ = json.Marshal(map[string]any{"error": "Global review rejected", "diagnostics": reviewDiags, "raw": response.Text})
				return uc.checkpointAI(ctx, &r, nil)
			}
			previousReview = response.Text
			r.Calls++
		}
		r.Review, _ = json.Marshal(review)
		trace.Record("review", "review", r.ReviewCycle, review)
		logger.InfoContext(ctx, "TRACFIN AI review finished", "blocking", blocking, "finding_count", len(review.Findings), "cycle", r.ReviewCycle)
		if blocking && r.ReviewCycle == 0 && len(repairs) > 0 {
			var sections []string
			for i := range repairs {
				sections = append(sections, r.Steps[i].Section)
			}
			logger.InfoContext(ctx, "TRACFIN AI review corrections started", "sections", sections)
			r.ReviewCycle++
			archived, _ := json.Marshal(r.Steps)
			var previousSteps []models.TracfinAIStep
			_ = json.Unmarshal(archived, &previousSteps)
			r.PreviousSteps = append(r.PreviousSteps, previousSteps)
			// Only the sections named by blocking findings are corrected, in place;
			// the others keep their accepted content and block IDs.
			aiScheduleReviewRepairs(&r, repairs)
			if err = uc.checkpointAI(ctx, &r, nil); err != nil {
				return err
			}
			continue
		}
		if needsInput || blocking || !aiValidationPassed(r.Validation) {
			r.Status = "needs_input"
		} else {
			r.Status = "completed"
		}
		trace.Record("finished", "", 0, r.Status)
		return uc.checkpointAI(ctx, &r, nil)
	}
}
func aiHasErrors(diags []models.TracfinDiagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// aiStepApplied reports whether a section's proposal was applied to the draft, possibly with
// reported missing data or conflicts.
func aiStepApplied(step models.TracfinAIStep) bool {
	return step.Status == "accepted" || step.Status == "partial"
}

// validateAIReview checks every finding and reports all invalid ones, so a single repair can fix them.
func validateAIReview(review tracfin.AIReview, r models.TracfinAIRun, d models.TracfinDraft, catalog tracfin.Catalog) (blocking bool, repairs map[int][]tracfin.AIReviewFinding, diags []models.TracfinDiagnostic) {
	repairs = map[int][]tracfin.AIReviewFinding{}
	reject := func(f tracfin.AIReviewFinding, code, message string) {
		diags = append(diags, aiError(code, "finding "+f.ID+": "+message))
	}
	analysis, _ := currentAIAnalysis(d)
	seen := map[string]bool{}
	for _, f := range review.Findings {
		if f.ID == "" || seen[f.ID] || strings.TrimSpace(f.Message) == "" {
			reject(f, "REVIEW_FINDING_INVALID", "id must be unique and non-empty, and message non-empty")
			continue
		}
		seen[f.ID] = true
		if err := tracfin.CheckAIEvidence(f.Message, f.Sources, aiSources(r)); err != nil {
			reject(f, "REVIEW_EVIDENCE_INVALID", err.Error())
			continue
		}
		if evidence := tracfin.ValidateAIProposalEvidence(tracfin.AISectionProposal{Values: []tracfin.AIValue{{Sources: f.Sources}}}, r.Context.Sources, r.AnalysisID, analysis.Text); len(evidence) > 0 {
			for _, e := range evidence {
				reject(f, "REVIEW_EVIDENCE_INVALID", e.Message+"; use an exact JSON Pointer inside that source's content")
			}
			continue
		}
		if f.Section == "analysis" {
			if f.FieldID != "" && f.FieldID != "/ds/analyse" {
				reject(f, "REVIEW_FIELD_INVALID", "an analysis finding may only target /ds/analyse")
				continue
			}
			blocking = blocking || f.Blocking
			continue
		}
		index := -1
		for j, s := range r.Steps {
			if s.Section == f.Section {
				index = j
			}
		}
		if index < 0 {
			reject(f, "REVIEW_SECTION_INVALID", "section must be one of attachments, persons, operations, declaration or analysis")
			continue
		}
		if _, known := catalog.Fields[f.FieldID]; f.FieldID != "" && (!known || !tracfin.AISectionOwns(f.Section, f.FieldID)) {
			reject(f, "REVIEW_FIELD_INVALID", "field_id "+f.FieldID+" is not a catalog field of section "+f.Section+"; use an exact catalog field ID or leave it empty")
			continue
		}
		if f.SuggestedAnalysis != "" {
			reject(f, "REVIEW_SUGGESTION_SCOPE", "suggested_analysis is only allowed on analysis findings")
			continue
		}
		if f.Blocking {
			blocking = true
			repairs[index] = append(repairs[index], f)
		}
	}
	return blocking, repairs, diags
}

// aiApplySettingsPrefill applies the organization settings mappings (declarant,
// contact) before any model call: these values are deterministic and never
// generated. Only single resolved candidates are applied; manual values and
// manual clears stay protected by ResolvePrefill, and an already applied
// candidate is not proposed again, so a resumed run changes nothing.
func aiApplySettingsPrefill(d models.TracfinDraft, catalog tracfin.Catalog) (models.TracfinDraft, int, error) {
	result := tracfin.ResolvePrefill(d.Configuration, d, tracfin.SourceSet{Records: map[tracfin.SourceKey]tracfin.SourceRecord{}, Decisions: map[string]models.TracfinDecisionSource{}})
	applied := 0
	for _, p := range result.Proposals {
		if p.Resolution != "resolved" || len(p.Candidates) != 1 {
			continue
		}
		c := p.Candidates[0]
		if c.Origin == nil || c.Origin.Source != "settings" || c.NeedsReview {
			continue
		}
		input := &models.TracfinFieldInput{Kind: c.Kind, Text: c.Text, Boolean: c.Boolean, Decimal: c.Decimal, ParentValueID: c.ParentValueID}
		next, err := tracfin.ApplyChanges(d, []tracfin.Change{{Action: "set_field", Field: p.Field, Value: input}}, catalog)
		if err != nil {
			return d, applied, err
		}
		attachTracfinOrigin(&next.Document.Root, p.Field, c.Origin, false)
		d = next
		applied++
	}
	return d, applied, nil
}
