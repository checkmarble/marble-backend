package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/checkmarble/marble-backend/utils"
)

const aiMaxEntities = 100

const aiEntityPlanInstructions = `Prepare a small explicit work list, not a declaration. Return tasks and exclusions arrays matching the schema.
Each task describes exactly ONE relevant natural/legal person or ONE distinct operation. Use a unique stable key for its future root block; section persons with catalog_id /ds/personnes/pp, /ds/personnes/pm or /ds/personnes/pnc; section operations with /ds/operations/operation or /ds/operations/operationEnCours. Never aggregate multiple transactions into one task. Order persons before operations, and each section in dependency order. Deduplicate snapshots/live records of the same object. Use relevant sources and analysis, never fabricate a person or transaction.
inventory groups source objects by table/object ID. Each inventory ID must occur exactly once, either in a task's inventory_id or in exclusions with a specific reason. Exclude accounts as standalone persons, while retaining them as evidence for the owning person. Exclude irrelevant objects or persons already completely represented by protected manual blocks; do not duplicate them. For persons/operations only evidenced by analysis or documents (no inventory object), inventory_id may be empty. Every task needs a factual explanation and precise source_id/locator evidence. Preserve all relevant entities; exclusions must be explicit and justified. Root objects and their supporting sources remain available during entity generation. Do not output fields, values, summaries or XML. Section containers and deferred relationships will be handled later; the backend computes the operations period and total. At most 100 tasks; operation and pending-operation counts must each respect the catalog limit of 30.`

const aiEntityInstructions = `Generate ONLY the entity specified by task. Return a complete AISectionProposal, with no has_more. The backend controls the work list.
Create exactly one root entity matching task.key and task.catalog_id, with its supported child blocks and attributes. Do not create any other person or operation. Use concise factual explanations and exact source evidence. If essential facts prevent representing this entity, explicitly report missing/conflicts instead of fabricating values.
completed_entities, accepted_blocks and references are a compact index of earlier results, NOT missing data. Those full results are retained by the backend. Never regenerate or copy those entities. Reuse existing section container keys from accepted_blocks or document.blocks. Create a section container only when absent. New keys must be unique, prefixed with task.key.
Do not amend earlier entities in an entity call. References may target earlier accepted keys or actual document block IDs. Defer links to not-yet-generated persons to section finalization. Do not invent IDs. For operations use actual declared persons/accounts from document.blocks, with their person context. Never emit the section-level operations period or total (dateDebut, dateFin, montantTotal of /ds/operations): the backend computes them from every accepted operation.`

const aiEntityFinalizationInstructions = `Finalize this section after all planned entities were processed. Return AISectionProposal without has_more. Do NOT create any new root persons or operations, repeat existing values, or recreate containers.
The compact accepted_blocks/completed_entities index identifies the generated entities; their complete values remain retained by the backend. Add only supported deferred relationship blocks/values needed to connect persons or operations. Reuse exact existing keys. All values still need precise evidence.
The section-level operations period (dateDebut, dateFin) and total (montantTotal) are computed by the backend from the accepted operations: never propose them. Keep pending/blocked operations distinct as supported by sources. If there are no declared financial operations, return an empty proposal and do not create an optional operations container. Report genuine gaps/conflicts explicitly. The backend assembles all retained proposals and checks references, catalog rules and totals.`

type aiInventoryEntity struct {
	ID        string   `json:"id"`
	SourceIDs []string `json:"source_ids"`
}

func aiEntityInventory(r models.TracfinAIRun) []aiInventoryEntity {
	groups := map[string][]string{}
	for _, s := range r.Context.Sources {
		var table, id string
		switch s.Kind {
		case "ingested_object":
			var obj struct {
				Table string `json:"table"`
				ID    string `json:"id"`
			}
			_ = json.Unmarshal(s.Content, &obj)
			table, id = obj.Table, obj.ID
		case "decision_snapshot":
			var obj struct {
				Table    string `json:"historical_type"`
				Snapshot struct {
					ID string `json:"object_id"`
				} `json:"snapshot"`
			}
			_ = json.Unmarshal(s.Content, &obj)
			table, id = obj.Table, obj.Snapshot.ID
		default:
			continue
		}
		key := s.ID
		if table != "" && id != "" {
			key = "object:" + table + ":" + id
		}
		groups[key] = append(groups[key], s.ID)
	}
	out := make([]aiInventoryEntity, 0, len(groups))
	for id, sources := range groups {
		sort.Strings(sources)
		out = append(out, aiInventoryEntity{ID: id, SourceIDs: sources})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func validateAIEntityPlan(plan tracfin.AIEntityPlan, r models.TracfinAIRun, analysis string) error {
	if len(plan.Tasks) > aiMaxEntities {
		return fmt.Errorf("entity plan exceeds %d tasks", aiMaxEntities)
	}
	inventory := map[string]bool{}
	objectSources := map[string]string{}
	for _, item := range aiEntityInventory(r) {
		inventory[item.ID] = false
		for _, source := range item.SourceIDs {
			objectSources[source] = item.ID
		}
	}
	keys := map[string]bool{}
	counts := map[string]int{}
	claim := func(id string) error {
		used, known := inventory[id]
		if !known {
			return fmt.Errorf("unknown inventory ID: %s", id)
		}
		if used {
			return fmt.Errorf("duplicate inventory ID: %s", id)
		}
		inventory[id] = true
		return nil
	}
	for _, task := range plan.Tasks {
		if task.Key == "" || strings.TrimSpace(task.Key) != task.Key || keys[task.Key] {
			return fmt.Errorf("empty or duplicate entity key")
		}
		keys[task.Key] = true
		allowed := task.Section == "persons" && (task.CatalogID == "/ds/personnes/pp" || task.CatalogID == "/ds/personnes/pm" || task.CatalogID == "/ds/personnes/pnc") || task.Section == "operations" && (task.CatalogID == "/ds/operations/operation" || task.CatalogID == "/ds/operations/operationEnCours")
		if !allowed {
			return fmt.Errorf("invalid entity section/catalog: %s", task.CatalogID)
		}
		counts[task.CatalogID]++
		if task.Section == "operations" && counts[task.CatalogID] > 30 {
			return fmt.Errorf("operation plan exceeds catalog cardinality")
		}
		if err := tracfin.CheckAIEvidence(task.Explanation, task.Sources, aiSources(r)); err != nil {
			return err
		}
		if ds := tracfin.ValidateAIProposalEvidence(tracfin.AISectionProposal{Values: []tracfin.AIValue{{Sources: task.Sources}}}, r.Context.Sources, r.AnalysisID, analysis); len(ds) > 0 {
			return fmt.Errorf("invalid entity evidence: %s", ds[0].Code)
		}
		if task.InventoryID != "" {
			if err := claim(task.InventoryID); err != nil {
				return err
			}
			cited := false
			for _, source := range task.Sources {
				if objectSources[source.SourceID] == task.InventoryID {
					cited = true
				}
			}
			if !cited {
				return fmt.Errorf("entity evidence does not cite its inventory object")
			}
		} else {
			for _, source := range task.Sources {
				if objectSources[source.SourceID] != "" {
					return fmt.Errorf("object-backed entity requires inventory_id")
				}
			}
		}
	}
	for _, excluded := range plan.Exclusions {
		if strings.TrimSpace(excluded.Reason) == "" {
			return fmt.Errorf("exclusion reason is required")
		}
		if err := claim(excluded.InventoryID); err != nil {
			return err
		}
	}
	for id, used := range inventory {
		if !used {
			return fmt.Errorf("unaccounted inventory object: %s", id)
		}
	}
	return nil
}
func aiCallLimit(r models.TracfinAIRun) int {
	if r.BatchingVersion != 2 {
		return aiMaxCalls
	}
	if r.CallBudget > 0 {
		return r.CallBudget
	}
	return 3 // Only planning is allowed before the validated work list exists.
}
func aiReadEntityPlan(r models.TracfinAIRun) (tracfin.AIEntityPlan, error) {
	var plan tracfin.AIEntityPlan
	err := tracfin.DecodeAIJSON(r.EntityPlan, &plan)
	return plan, err
}
func aiSectionTasks(r models.TracfinAIRun, section string) ([]tracfin.AIEntityTask, error) {
	plan, err := aiReadEntityPlan(r)
	if err != nil {
		return nil, err
	}
	var tasks []tracfin.AIEntityTask
	for _, task := range plan.Tasks {
		if task.Section == section {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

type aiEntityBlockIndex struct {
	Key       string `json:"key"`
	ParentKey string `json:"parent_key,omitempty"`
	CatalogID string `json:"catalog_id"`
	// Only evidence identities, never the generated person's scalar values.
	SourceIDs   []string `json:"source_ids,omitempty"`
	InventoryID string   `json:"inventory_id,omitempty"`
}
type aiEntityReferenceIndex struct {
	OwnerKey  string `json:"owner_key"`
	FieldID   string `json:"field_id"`
	TargetKey string `json:"target_key"`
	PersonKey string `json:"person_key,omitempty"`
}
type aiEntityWorkInput struct {
	References        []aiEntityReferenceIndex `json:"references"`
	Mode              string                   `json:"mode"`
	Task              *tracfin.AIEntityTask    `json:"task,omitempty"`
	Index             int                      `json:"index"`
	Total             int                      `json:"total"`
	CompletedEntities []string                 `json:"completed_entities"`
	AcceptedBlocks    []aiEntityBlockIndex     `json:"accepted_blocks"`
	Instructions      string                   `json:"instructions"`
}

func aiEntityWork(r models.TracfinAIRun, step models.TracfinAIStep) (aiEntityWorkInput, error) {
	tasks, err := aiSectionTasks(r, step.Section)
	if err != nil {
		return aiEntityWorkInput{}, err
	}
	accepted, err := aiAcceptedBatches(step)
	if err != nil {
		return aiEntityWorkInput{}, err
	}
	index := len(step.Batches)
	if index > len(tasks) {
		return aiEntityWorkInput{}, fmt.Errorf("entity work cursor exceeds plan")
	}
	work := aiEntityWorkInput{Mode: "finalize", Index: index + 1, Total: len(tasks) + 1, CompletedEntities: []string{}, AcceptedBlocks: []aiEntityBlockIndex{}, Instructions: r.PromptSnapshot["builtin/tracfin/entity_finalize_v2"]}
	if index < len(tasks) {
		work.Mode = "entity"
		work.Task = &tasks[index]
		work.Instructions = r.PromptSnapshot["builtin/tracfin/entity_v2"]
	}
	if work.Instructions == "" {
		return work, fmt.Errorf("frozen entity protocol instructions unavailable")
	}
	for _, task := range tasks[:min(index, len(tasks))] {
		work.CompletedEntities = append(work.CompletedEntities, task.Key)
	}
	for _, block := range accepted.Blocks {
		item := aiEntityBlockIndex{Key: block.Key, ParentKey: block.ParentKey, CatalogID: block.CatalogID}
		for _, task := range tasks {
			if task.Key == block.Key {
				item.InventoryID = task.InventoryID
				for _, s := range task.Sources {
					item.SourceIDs = append(item.SourceIDs, s.SourceID)
				}
			}
		}
		seen := map[string]bool{}
		for _, id := range item.SourceIDs {
			seen[id] = true
		}
		for _, value := range accepted.Values {
			if value.BlockKey == block.Key {
				for _, source := range value.Sources {
					seen[source.SourceID] = true
				}
			}
		}
		item.SourceIDs = nil
		for id := range seen {
			item.SourceIDs = append(item.SourceIDs, id)
		}
		sort.Strings(item.SourceIDs)
		work.AcceptedBlocks = append(work.AcceptedBlocks, item)
	}
	work.References = []aiEntityReferenceIndex{}
	for _, value := range accepted.Values {
		if value.Kind == "reference" {
			work.References = append(work.References, aiEntityReferenceIndex{OwnerKey: value.BlockKey, FieldID: value.FieldID, TargetKey: value.ReferenceKey, PersonKey: value.PersonReferenceKey})
		}
	}
	return work, nil
}
func aiEntityDocumentIndex(d models.TracfinDraft) any {
	blocks := []aiEntityBlockIndex{}
	var walk func(models.TracfinBlock, string)
	walk = func(b models.TracfinBlock, parent string) {
		item := aiEntityBlockIndex{Key: b.ID, ParentKey: parent, CatalogID: b.CatalogID}
		// Keep source identities to resolve each accepted person/account without copying values.
		seen := map[string]bool{}
		var evidence func(models.TracfinBlock)
		evidence = func(child models.TracfinBlock) {
			for _, values := range child.Fields {
				for _, value := range values {
					if value.Origin != nil {
						for _, source := range value.Origin.Evidence {
							seen[source.SourceID] = true
						}
					}
				}
			}
			for _, c := range child.Children {
				evidence(c)
			}
		}
		if b.CatalogID == "/ds/personnes/pp" || b.CatalogID == "/ds/personnes/pm" || strings.Contains(b.CatalogID, "/referenceFinanciere") {
			evidence(b)
		}
		for id := range seen {
			item.SourceIDs = append(item.SourceIDs, id)
		}
		sort.Strings(item.SourceIDs)
		blocks = append(blocks, item)
		for _, c := range b.Children {
			walk(c, b.ID)
		}
	}
	walk(d.Document.Root, "")
	return map[string]any{"root_id": d.Document.Root.ID, "blocks": blocks}
}
func aiDecodeEntityBatch(raw string, r models.TracfinAIRun, step models.TracfinAIStep) (tracfin.AISectionProposal, tracfin.AISectionProposal, bool, []models.TracfinDiagnostic) {
	var page tracfin.AISectionProposal
	fail := func(code, msg string) (tracfin.AISectionProposal, tracfin.AISectionProposal, bool, []models.TracfinDiagnostic) {
		return page, page, false, []models.TracfinDiagnostic{aiError(code, msg)}
	}
	if err := tracfin.DecodeAIJSON([]byte(raw), &page); err != nil {
		return fail("JSON_INVALID", err.Error())
	}
	if step.Section == "operations" {
		tracfin.DropAIOperationAggregates(&page)
	}
	work, err := aiEntityWork(r, step)
	if err != nil {
		return fail("ENTITY_PLAN_STATE", err.Error())
	}
	roots := 0
	own := map[string]bool{}
	for _, block := range page.Blocks {
		own[block.Key] = true
		switch block.CatalogID {
		case "/ds/personnes/pp", "/ds/personnes/pm", "/ds/personnes/pnc", "/ds/operations/operation", "/ds/operations/operationEnCours":
			roots++
			if work.Task == nil || block.Key != work.Task.Key || block.CatalogID != work.Task.CatalogID {
				return fail("ENTITY_SCOPE", "Generate only the exact planned entity; finalization cannot create root entities")
			}
		}
	}
	if work.Task != nil {
		if roots == 0 && len(page.Missing)+len(page.Conflicts) == 0 {
			return fail("ENTITY_MISSING", "The planned entity is absent without an explicit missing fact or conflict")
		}
		if roots > 1 {
			return fail("ENTITY_SCOPE", "Only one root entity is allowed per call")
		}
		parents := map[string]string{}
		for _, block := range page.Blocks {
			parents[block.Key] = block.ParentKey
		}
		for _, block := range page.Blocks {
			if block.Key == work.Task.Key || block.CatalogID == "/ds/personnes" || block.CatalogID == "/ds/operations" {
				continue
			}
			current := block.Key
			for hops := 0; hops <= len(page.Blocks) && current != work.Task.Key; hops++ {
				current = parents[current]
				if current == "" {
					break
				}
			}
			if current != work.Task.Key {
				return fail("ENTITY_SCOPE", "An entity call cannot add children to an earlier entity")
			}
		}
		for _, value := range page.Values {
			if !own[value.BlockKey] {
				return fail("ENTITY_SCOPE", "An entity call cannot amend an earlier entity")
			}
		}
	}
	merged, diags := aiMergeBatch(page, step)
	return merged, page, work.Task != nil, diags
}

func (uc TracfinUsecase) prepareAIEntityPlan(ctx context.Context, r *models.TracfinAIRun, trace *tracfin.AITrace) (bool, error) {
	if len(r.EntityPlan) > 0 {
		return true, nil
	}
	logger := utils.LoggerFromContext(ctx).With("run_id", r.ID, "case_id", r.CaseID)
	for len(r.PlanAttempts) < 3 && r.Calls < aiCallLimit(*r) {
		d, err := uc.GetDraft(ctx, r.CaseID)
		if err != nil {
			return false, err
		}
		previous := ""
		var diagnostics []models.TracfinDiagnostic
		if len(r.PlanAttempts) > 0 {
			last := r.PlanAttempts[len(r.PlanAttempts)-1]
			previous = last.Raw
			diagnostics = last.Diagnostics
		}
		r.PlanAttempts = append(r.PlanAttempts, models.TracfinAIAttempt{Number: len(r.PlanAttempts) + 1, StartedAt: time.Now().UTC(), Status: "started"})
		r.Calls++
		if err = uc.checkpointAI(ctx, r, nil); err != nil {
			return false, err
		}
		response, callErr := uc.callAI(ctx, r, d, "entity_plan", previous, diagnostics, nil, trace)
		if aiInterrupted(ctx, callErr) {
			r.PlanAttempts = r.PlanAttempts[:len(r.PlanAttempts)-1]
			r.Calls--
			return false, uc.releaseInterruptedAI(ctx, r, callErr)
		}
		attempt := &r.PlanAttempts[len(r.PlanAttempts)-1]
		attempt.Raw = response.Text
		attempt.Status = "rejected"
		var plan tracfin.AIEntityPlan
		switch {
		case callErr != nil:
			attempt.Diagnostics = []models.TracfinDiagnostic{aiError("ENTITY_PLAN_UNAVAILABLE", "Entity planner failed: "+aiModelFailure(callErr)+"; attempt consumed")}
		case response.FinishReason == "max_tokens":
			attempt.Diagnostics = []models.TracfinDiagnostic{aiError("OUTPUT_TRUNCATED", "Entity work list exceeded output budget")}
		default:
			err = tracfin.DecodeAIJSON([]byte(response.Text), &plan)
			if err == nil {
				analysis, _ := currentAIAnalysis(d)
				err = validateAIEntityPlan(plan, *r, analysis.Text)
			}
			if err != nil {
				attempt.Diagnostics = []models.TracfinDiagnostic{aiError("ENTITY_PLAN_INVALID", err.Error())}
			}
		}
		if errors.Is(callErr, errTracfinAIContextBudget) {
			r.Calls--
			r.Status = "context_budget"
			attempt.Diagnostics = []models.TracfinDiagnostic{aiError("CONTEXT_BUDGET", callErr.Error())}
			return false, uc.checkpointAI(ctx, r, nil)
		}
		trace.Record("plan_validation", "entity_plan", attempt.Number, attempt.Diagnostics)
		if len(attempt.Diagnostics) == 0 {
			attempt.Status = "accepted"
			r.EntityPlan, _ = json.Marshal(plan)
			// Two generation cycles, three attempts for each entity/four fixed units,
			// three planning attempts and two global reviews of up to three attempts. The wall deadline still applies.
			r.CallBudget = 3 + 6*(len(plan.Tasks)+4) + 6
			logger.InfoContext(ctx, "TRACFIN AI entity plan accepted", "entities", len(plan.Tasks), "excluded_objects", len(plan.Exclusions), "call_budget", r.CallBudget)
			return true, uc.checkpointAI(ctx, r, nil)
		}
		logger.WarnContext(ctx, "TRACFIN AI entity plan rejected", "attempt", attempt.Number, "diagnostics", attempt.Diagnostics)
		if err = uc.checkpointAI(ctx, r, nil); err != nil {
			return false, err
		}
	}
	r.Status = "needs_input"
	if len(r.Steps) > 0 {
		r.Steps[0].Status = "blocked"
		r.Steps[0].Diagnostics = r.PlanAttempts[len(r.PlanAttempts)-1].Diagnostics
	}
	return false, uc.checkpointAI(ctx, r, nil)
}
