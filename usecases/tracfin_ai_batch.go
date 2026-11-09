package usecases

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
)

const aiMaxBatches = 8

// This protocol is frozen by BatchingVersion; legacy executions keep their original schema.
const aiBatchInstructions = `Return one complete JSON object matching the response schema, including has_more (boolean).
Generate at most max_entities NEW persons (pp/pm) or operations, with all their supported children and attributes. Finish every entity in this batch; never cut JSON or split an entity across batches. Use concise factual explanations.
accepted_proposal contains immutable validated earlier batches. Do not repeat their blocks, values or entities. Reuse their exact temporary keys as parent_key/reference_key/parent_value_key when needed. Create section containers only once. Keys must be unique across the entire section, for example batch-number-prefixed keys.
Keep all relevant source entities in scope. has_more=true means other relevant entities remain; has_more=false means the ENTIRE section is covered, not merely this batch. Missing facts must be reported, never silently skipped. Do not use has_more=true with an empty proposal.
For operations, emit section-level dates, counts and totals only in the FINAL batch, calculated over ALL declared operations in accepted_proposal and this batch. Earlier batches must not emit provisional totals.
Use the accepted persons/accounts from document for operation references. Never invent identifiers or facts. The immutable context and analysis remain authoritative. This batch contract also applies to repairs; repair ONLY the current batch, never accepted_proposal.`

func aiBatchStep(r *models.TracfinAIRun, section string) *models.TracfinAIStep {
	if (r.BatchingVersion != 1 && r.BatchingVersion != 2) || (section != "persons" && section != "operations") {
		return nil
	}
	for i := range r.Steps {
		if r.Steps[i].Section == section {
			return &r.Steps[i]
		}
	}
	return nil
}
func aiBatchSize(step models.TracfinAIStep) int {
	if step.BatchSize > 0 {
		return step.BatchSize
	}
	if step.Section == "persons" {
		return 2
	}
	return 3
}
func aiAcceptedBatches(step models.TracfinAIStep) (tracfin.AISectionProposal, error) {
	p := tracfin.AISectionProposal{Blocks: []tracfin.AIBlock{}, Values: []tracfin.AIValue{}, Attachments: []tracfin.AIAttachment{}, Missing: []string{}, Conflicts: []string{}}
	for _, raw := range step.Batches {
		var batch tracfin.AISectionProposal
		if err := tracfin.DecodeAIJSON(raw, &batch); err != nil {
			return p, fmt.Errorf("invalid persisted batch: %w", err)
		}
		p.Blocks = append(p.Blocks, batch.Blocks...)
		p.Values = append(p.Values, batch.Values...)
		p.Attachments = append(p.Attachments, batch.Attachments...)
		p.Missing = append(p.Missing, batch.Missing...)
		p.Conflicts = append(p.Conflicts, batch.Conflicts...)
	}
	return p, nil
}
func aiDecodeBatch(raw string, step models.TracfinAIStep) (tracfin.AISectionProposal, tracfin.AISectionProposal, bool, []models.TracfinDiagnostic) {
	var batch tracfin.AISectionBatch
	if err := tracfin.DecodeAIJSON([]byte(raw), &batch); err != nil {
		return batch.AISectionProposal, batch.AISectionProposal, false, []models.TracfinDiagnostic{aiError("JSON_INVALID", err.Error())}
	}
	page := batch.AISectionProposal
	fail := func(code, msg string) (tracfin.AISectionProposal, tracfin.AISectionProposal, bool, []models.TracfinDiagnostic) {
		return page, page, false, []models.TracfinDiagnostic{aiError(code, msg)}
	}
	more := *batch.HasMore
	if len(step.Batches) >= aiMaxBatches || (more && len(step.Batches)+1 >= aiMaxBatches) {
		return fail("BATCH_LIMIT", "Section exceeds the bounded batch budget; narrow the case scope")
	}
	if more && len(page.Blocks)+len(page.Values) == 0 {
		return fail("BATCH_NO_PROGRESS", "A non-final batch must contain new blocks or values")
	}
	if step.Section == "operations" {
		tracfin.DropAIOperationAggregates(&page)
	}
	entities := 0
	for _, b := range page.Blocks {
		if b.CatalogID == "/ds/personnes/pp" || b.CatalogID == "/ds/personnes/pm" || b.CatalogID == "/ds/operations/operation" || b.CatalogID == "/ds/operations/operationEnCours" {
			entities++
		}
	}
	if entities > aiBatchSize(step) {
		return fail("BATCH_SIZE", "Batch contains more entities than requested")
	}
	merged, diags := aiMergeBatch(page, step)
	return merged, page, more, diags
}

func aiMergeBatch(page tracfin.AISectionProposal, step models.TracfinAIStep) (tracfin.AISectionProposal, []models.TracfinDiagnostic) {
	fail := func(code, message string) (tracfin.AISectionProposal, []models.TracfinDiagnostic) {
		return page, []models.TracfinDiagnostic{aiError(code, message)}
	}
	merged, err := aiAcceptedBatches(step)
	if err != nil {
		return fail("BATCH_STATE", err.Error())
	}
	keys := map[string]bool{}
	for _, b := range merged.Blocks {
		keys[b.Key] = true
	}
	for _, v := range merged.Values {
		keys[v.Key] = true
	}
	for _, b := range page.Blocks {
		if keys[b.Key] {
			return fail("BATCH_KEY_COLLISION", "A batch must not redefine an accepted key")
		}
		keys[b.Key] = true
	}
	for _, v := range page.Values {
		if keys[v.Key] {
			return fail("BATCH_KEY_COLLISION", "A batch must not redefine an accepted key")
		}
		keys[v.Key] = true
	}
	merged.Blocks = append(merged.Blocks, page.Blocks...)
	merged.Values = append(merged.Values, page.Values...)
	merged.Attachments = append(merged.Attachments, page.Attachments...)
	merged.Missing = append(merged.Missing, page.Missing...)
	merged.Conflicts = append(merged.Conflicts, page.Conflicts...)
	return merged, nil
}
