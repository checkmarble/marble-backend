package usecases

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/stretchr/testify/require"
)

func enableAIEntities(t *testing.T, r *models.TracfinAIRun) {
	t.Helper()
	enableAIBatching(t, r)
	r.PromptSnapshot["builtin/tracfin/entity_plan_v2"] = aiEntityPlanInstructions + "\nInput JSON:\n{{.Input}}"
	r.PromptSnapshot["builtin/tracfin/entity_v2"] = aiEntityInstructions
	r.PromptSnapshot["builtin/tracfin/entity_finalize_v2"] = aiEntityFinalizationInstructions
	raw, err := json.Marshal(r)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	fields["batching_version"] = json.RawMessage(`2`)
	raw, err = json.Marshal(fields)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, r))
}
func entityPlanJSON(analysisID string) string {
	raw, _ := json.Marshal(map[string]any{"tasks": []any{map[string]any{"key": "person", "section": "persons", "catalog_id": "/ds/personnes/pp", "inventory_id": "", "explanation": "Person described in analyst fixture", "sources": []any{map[string]string{"source_id": "analysis:" + analysisID, "locator": "text"}}}}, "exclusions": []any{}})
	return string(raw)
}
func TestAIEntityGenerationUsesExplicitTaskWithoutContinuationFlag(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	enableAIEntities(t, &runs.run)
	llm.responses = []string{entityPlanJSON(runs.run.AnalysisID), emptyAIProposal, peopleProposal(runs.run.AnalysisID), emptyAIProposal, emptyAIProposal, emptyAIProposal, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, 7, llm.calls)
	require.Equal(t, "accepted", runs.run.Steps[1].Status)
	require.Len(t, runs.run.Steps[1].Batches, 2, "one entity and finalization")
	var input map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(llm.prompts[2]), &input))
	require.Contains(t, string(input["entity_work"]), `"key":"person"`)
	require.Equal(t, draft.draft.AI.Analyses[0].Text, *draft.draft.Document.Root.Fields["/ds/analyse"][0].Text)
}

func TestAIEntityPlannerRejectsUnknownInventoryBeforeGenerating(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	enableAIEntities(t, &runs.run)
	bad := `{"tasks":[],"exclusions":[{"inventory_id":"unknown","reason":"ignored"}]}`
	llm.responses = []string{bad, bad, bad}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, 3, llm.calls)
	require.Equal(t, "needs_input", runs.run.Status)
	require.Empty(t, runs.run.Steps[0].Attempts)
}

func TestAIEntityInventoryDeduplicatesSnapshotsAndRequiresCoverage(t *testing.T) {
	r := models.TracfinAIRun{Context: models.TracfinAIContext{Sources: []models.TracfinAISource{
		{ID: "object:transfer:T1", Kind: "ingested_object", Content: json.RawMessage(`{"table":"transfer","id":"T1"}`)},
		{ID: "snapshot:d1", Kind: "decision_snapshot", Content: json.RawMessage(`{"historical_type":"transfer","snapshot":{"object_id":"T1"}}`)},
		{ID: "snapshot:d2", Kind: "decision_snapshot", Content: json.RawMessage(`{"historical_type":"transfer","snapshot":{"object_id":"T1"}}`)},
	}}}
	inventory := aiEntityInventory(r)
	require.Len(t, inventory, 1)
	require.Len(t, inventory[0].SourceIDs, 3)
	plan := tracfin.AIEntityPlan{Tasks: []tracfin.AIEntityTask{}, Exclusions: []tracfin.AIEntityExclusion{}}
	require.ErrorContains(t, validateAIEntityPlan(plan, r, ""), "unaccounted")
	plan.Exclusions = append(plan.Exclusions, tracfin.AIEntityExclusion{InventoryID: inventory[0].ID, Reason: "Explicit synthetic exclusion"})
	require.NoError(t, validateAIEntityPlan(plan, r, ""))
}

func TestAIEntityNextPersonReceivesCompactIndexWithoutPreviousDetails(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	enableAIEntities(t, &runs.run)
	var plan tracfin.AIEntityPlan
	require.NoError(t, json.Unmarshal([]byte(entityPlanJSON(runs.run.AnalysisID)), &plan))
	next := plan.Tasks[0]
	next.Key = "person2"
	plan.Tasks = append(plan.Tasks, next)
	planRaw, err := json.Marshal(plan)
	require.NoError(t, err)
	var first tracfin.AISectionProposal
	require.NoError(t, json.Unmarshal([]byte(peopleProposal(runs.run.AnalysisID)), &first))
	marker := "GENERATED_PRIVATE_DETAIL_NOT_NEEDED_IN_NEXT_CALL"
	first.Values = append(first.Values, tracfin.AIValue{Key: "private-description", BlockKey: "person", FieldID: "/ds/personnes/pp/description", Kind: "text", Text: &marker, Explanation: "Synthetic analyst fixture", Sources: plan.Tasks[0].Sources})
	rawFirst, err := json.Marshal(first)
	require.NoError(t, err)
	var second tracfin.AISectionProposal
	require.NoError(t, json.Unmarshal([]byte(peopleProposal(runs.run.AnalysisID)), &second))
	second.Blocks = second.Blocks[1:]
	for i := range second.Blocks {
		if second.Blocks[i].Key == "person" {
			second.Blocks[i].Key = "person2"
		}
		if second.Blocks[i].Key == "civil" {
			second.Blocks[i].Key = "civil2"
		}
		if second.Blocks[i].ParentKey == "person" {
			second.Blocks[i].ParentKey = "person2"
		}
	}
	for i := range second.Values {
		v := &second.Values[i]
		v.Key += "2"
		if v.BlockKey == "person" {
			v.BlockKey = "person2"
		}
		if v.BlockKey == "civil" {
			v.BlockKey = "civil2"
		}
	}
	rawSecond, err := json.Marshal(second)
	require.NoError(t, err)
	llm.responses = []string{string(planRaw), emptyAIProposal, string(rawFirst), string(rawSecond), emptyAIProposal, emptyAIProposal, emptyAIProposal, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, "accepted", runs.run.Steps[1].Status)
	require.Len(t, runs.run.Steps[1].Batches, 3)
	require.Contains(t, llm.prompts[3], `"completed_entities":["person"]`)
	require.Contains(t, llm.prompts[3], `"key":"person2"`)
	require.NotContains(t, llm.prompts[3], marker)
	require.NotContains(t, llm.prompts[3], `"accepted_proposal"`)
	require.Contains(t, string(runs.run.Steps[1].Candidate), marker, "backend retains all details for final assembly")
	require.Greater(t, runs.run.CallBudget, 26)
}

func TestAIEntityResumeRetainsPlanAndCompletedEntities(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	enableAIEntities(t, &runs.run)
	llm.responses = []string{entityPlanJSON(runs.run.AnalysisID), emptyAIProposal, peopleProposal(runs.run.AnalysisID), emptyAIProposal, emptyAIProposal, emptyAIProposal, `{"findings":[]}`}
	llm.before = func(call int) {
		if call == 3 {
			panic("worker restart before finalization")
		}
	}
	require.Panics(t, func() { _ = uc.GenerateAI(ctx, "case", "run") })
	raw, err := json.Marshal(runs.run)
	require.NoError(t, err)
	var restored models.TracfinAIRun
	require.NoError(t, json.Unmarshal(raw, &restored))
	runs.run = restored
	require.Len(t, runs.run.PlanAttempts, 1)
	require.Len(t, runs.run.Steps[1].Batches, 1)
	llm.before = nil
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Len(t, runs.run.PlanAttempts, 1, "planning is not replayed")
	require.Len(t, runs.run.Steps[1].Batches, 2)
	require.Equal(t, "accepted", runs.run.Steps[1].Status)
	require.Equal(t, 8, runs.run.Calls, "crashed call still consumes reserved budget")
	require.Equal(t, 7, llm.calls)
}

func TestAIEntityDecoderRejectsWrongEntityAndEarlyCompletion(t *testing.T) {
	_, _, runs, _, _ := generationFixture(t)
	enableAIEntities(t, &runs.run)
	runs.run.EntityPlan = json.RawMessage(entityPlanJSON(runs.run.AnalysisID))
	step := models.TracfinAIStep{Section: "persons"}
	_, _, _, ds := aiDecodeEntityBatch(emptyAIProposal, runs.run, step)
	require.Equal(t, "ENTITY_MISSING", ds[0].Code)
	var p tracfin.AISectionProposal
	require.NoError(t, json.Unmarshal([]byte(peopleProposal(runs.run.AnalysisID)), &p))
	p.Blocks[1].Key = "wrong-person"
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, _, _, ds = aiDecodeEntityBatch(string(raw), runs.run, step)
	require.Equal(t, "ENTITY_SCOPE", ds[0].Code)
	_, _, _, ds = aiDecodeEntityBatch(batchJSON(emptyAIProposal, false), runs.run, step)
	require.Equal(t, "JSON_INVALID", ds[0].Code, "v2 accepts no model-controlled continuation flag")
}

func TestAIEntityBudgetAllowsMoreThanTwentySixUnitCalls(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	enableAIEntities(t, &runs.run)
	var plan tracfin.AIEntityPlan
	require.NoError(t, json.Unmarshal([]byte(entityPlanJSON(runs.run.AnalysisID)), &plan))
	template := plan.Tasks[0]
	plan.Tasks = nil
	people := make([]string, 0, 24)
	for i := 0; i < 24; i++ {
		suffix := fmt.Sprint(i)
		task := template
		task.Key = "person" + suffix
		plan.Tasks = append(plan.Tasks, task)
		var p tracfin.AISectionProposal
		require.NoError(t, json.Unmarshal([]byte(peopleProposal(runs.run.AnalysisID)), &p))
		if i > 0 {
			p.Blocks = p.Blocks[1:]
		}
		for j := range p.Blocks {
			b := &p.Blocks[j]
			if b.Key == "person" {
				b.Key = task.Key
			}
			if b.Key == "civil" {
				b.Key = "civil" + suffix
			}
			if b.ParentKey == "person" {
				b.ParentKey = task.Key
			}
		}
		for j := range p.Values {
			v := &p.Values[j]
			v.Key += suffix
			if v.BlockKey == "person" {
				v.BlockKey = task.Key
			}
			if v.BlockKey == "civil" {
				v.BlockKey = "civil" + suffix
			}
		}
		raw, err := json.Marshal(p)
		require.NoError(t, err)
		people = append(people, string(raw))
	}
	raw, err := json.Marshal(plan)
	require.NoError(t, err)
	llm.responses = []string{string(raw), emptyAIProposal}
	llm.responses = append(llm.responses, people...)
	llm.responses = append(llm.responses, emptyAIProposal, emptyAIProposal, emptyAIProposal, `{"findings":[]}`)
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, 30, llm.calls)
	require.Equal(t, 30, runs.run.Calls)
	require.Equal(t, 177, runs.run.CallBudget)
	require.Equal(t, "accepted", runs.run.Steps[1].Status)
	require.NotEqual(t, "exhausted", runs.run.Status)
}
