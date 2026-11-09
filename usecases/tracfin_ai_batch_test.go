package usecases

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/stretchr/testify/require"
)

const emptyAIProposal = `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`

func enableAIBatching(t *testing.T, r *models.TracfinAIRun) {
	t.Helper()
	raw, err := json.Marshal(r)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	fields["batching_version"] = json.RawMessage(`1`)
	raw, err = json.Marshal(fields)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, r))
}

func batchJSON(proposal string, more bool) string {
	var fields map[string]any
	_ = json.Unmarshal([]byte(proposal), &fields)
	fields["has_more"] = more
	raw, _ := json.Marshal(fields)
	return string(raw)
}

func peopleProposal(analysisID string) string {
	return strings.ReplaceAll(`{"blocks":[{"key":"people","parent_key":"root","catalog_id":"/ds/personnes"},{"key":"person","parent_key":"people","catalog_id":"/ds/personnes/pp"},{"key":"civil","parent_key":"person","catalog_id":"/ds/personnes/pp/etatCivil"}],"values":[{"key":"implication","block_key":"person","field_id":"/ds/personnes/pp/implication","kind":"text","text":"Personne soupçonnée / Personne impliquée","explanation":"Synthetic fixture subject","sources":[{"source_id":"analysis:ANALYSIS","locator":"text"}]},{"key":"name","block_key":"civil","field_id":"/ds/personnes/pp/etatCivil/nomDusage","kind":"text","text":"Analyse","explanation":"From human analysis fixture","sources":[{"source_id":"analysis:ANALYSIS","locator":"text"}]},{"key":"first","block_key":"civil","field_id":"/ds/personnes/pp/etatCivil/prenom","kind":"text","text":"humaine","explanation":"From human analysis fixture","sources":[{"source_id":"analysis:ANALYSIS","locator":"text"}]}],"attachments":[],"missing":[],"conflicts":[]}`, "ANALYSIS", analysisID)
}

func TestAIGenerationCombinesValidBatchesBeforeApplyingSection(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	enableAIBatching(t, &runs.run)
	llm.responses = []string{emptyAIProposal, batchJSON(peopleProposal(runs.run.AnalysisID), true), batchJSON(emptyAIProposal, false), batchJSON(emptyAIProposal, false), emptyAIProposal, `{"findings":[]}`}
	llm.before = func(call int) {
		if call == 2 {
			for _, child := range draft.draft.Document.Root.Children {
				require.NotEqual(t, "/ds/personnes", child.CatalogID, "intermediate batch must not modify draft")
			}
		}
	}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, 6, llm.calls)
	require.Equal(t, "accepted", runs.run.Steps[1].Status)
	var p tracfin.AISectionProposal
	require.NoError(t, json.Unmarshal(runs.run.Steps[1].Candidate, &p))
	require.Len(t, p.Blocks, 3)
	require.Equal(t, "people", p.Blocks[0].Key)
	var peopleCount int
	tracfin.WalkBlocks(draft.draft.Document.Root, func(b models.TracfinBlock) {
		if b.CatalogID == "/ds/personnes" {
			peopleCount++
		}
	})
	require.Equal(t, 1, peopleCount)
	require.Equal(t, draft.draft.AI.Analyses[0].Text, *draft.draft.Document.Root.Fields["/ds/analyse"][0].Text)
}

func TestAIGenerationBatchRejectsMissingCompletionFlagAndEmptyContinuation(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	enableAIBatching(t, &runs.run)
	llm.responses = []string{emptyAIProposal, emptyAIProposal, batchJSON(emptyAIProposal, true), batchJSON(emptyAIProposal, false), batchJSON(emptyAIProposal, false), emptyAIProposal, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Len(t, runs.run.Steps[1].Attempts, 3)
	require.Equal(t, "JSON_INVALID", runs.run.Steps[1].Attempts[0].Diagnostics[0].Code)
	require.Equal(t, "BATCH_NO_PROGRESS", runs.run.Steps[1].Attempts[1].Diagnostics[0].Code)
	require.Equal(t, "accepted", runs.run.Steps[1].Status)
}

func TestAIGenerationBatchDoesNotOverwritePreviouslyAcceptedKeys(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	enableAIBatching(t, &runs.run)
	llm.responses = []string{emptyAIProposal, batchJSON(peopleProposal(runs.run.AnalysisID), true), batchJSON(peopleProposal(runs.run.AnalysisID), false), batchJSON(peopleProposal(runs.run.AnalysisID), false), batchJSON(peopleProposal(runs.run.AnalysisID), false), emptyAIProposal, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, "exhausted", runs.run.Steps[1].Status)
	require.Len(t, runs.run.Steps[1].Attempts, 4, "three retries allowed for current batch only")
	require.Equal(t, "BATCH_KEY_COLLISION", runs.run.Steps[1].Attempts[1].Diagnostics[0].Code)
	require.Equal(t, "blocked", runs.run.Steps[2].Status)
}

func TestAIGenerationTruncatedBatchShrinksAndKeepsValidatedBatches(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	enableAIBatching(t, &runs.run)
	llm.responses = []string{emptyAIProposal, batchJSON(peopleProposal(runs.run.AnalysisID), true), `{"blocks":[`, batchJSON(emptyAIProposal, false), batchJSON(emptyAIProposal, false), emptyAIProposal, `{"findings":[]}`}
	llm.finishReasons = map[int]string{2: "max_tokens"}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, "accepted", runs.run.Steps[1].Status)
	require.Equal(t, 1, runs.run.Steps[1].BatchSize)
	require.Len(t, runs.run.Steps[1].Batches, 2)
	require.Equal(t, "OUTPUT_TRUNCATED", runs.run.Steps[1].Attempts[1].Diagnostics[0].Code)
	require.Len(t, runs.run.Steps[1].Attempts, 3)
	var input struct {
		Batch struct {
			Number           int                       `json:"number"`
			MaxEntities      int                       `json:"max_entities"`
			AcceptedProposal tracfin.AISectionProposal `json:"accepted_proposal"`
		} `json:"batch"`
	}
	require.NoError(t, json.Unmarshal([]byte(llm.prompts[3]), &input))
	require.Equal(t, 2, input.Batch.Number)
	require.Equal(t, 1, input.Batch.MaxEntities)
	require.Len(t, input.Batch.AcceptedProposal.Blocks, 3)
	require.Equal(t, 7, llm.calls)
}

func TestAIGenerationResumesAfterCrashWithoutReplayingValidatedBatch(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	enableAIBatching(t, &runs.run)
	llm.responses = []string{emptyAIProposal, batchJSON(peopleProposal(runs.run.AnalysisID), true), batchJSON(emptyAIProposal, false), batchJSON(emptyAIProposal, false), emptyAIProposal, `{"findings":[]}`}
	llm.before = func(call int) {
		if call == 2 {
			panic("simulated worker crash")
		}
	}
	require.Panics(t, func() { _ = uc.GenerateAI(ctx, "case", "run") })
	// Rehydrate the last checkpoint just as a new worker does from the database.
	raw, err := json.Marshal(runs.run)
	require.NoError(t, err)
	var restored models.TracfinAIRun
	require.NoError(t, json.Unmarshal(raw, &restored))
	runs.run = restored
	require.Len(t, runs.run.Steps[1].Batches, 1)
	require.Equal(t, 1, runs.run.Steps[1].BatchAttemptStart)
	llm.before = nil
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, "accepted", runs.run.Steps[1].Status)
	require.Len(t, runs.run.Steps[1].Batches, 2)
	require.Len(t, runs.run.Steps[1].Attempts, 3, "interrupted attempt still consumes a retry")
	require.Equal(t, 7, runs.run.Calls, "reservation before crash must not be refunded")
	require.Equal(t, 6, llm.calls)
	var count int
	tracfin.WalkBlocks(draft.draft.Document.Root, func(b models.TracfinBlock) {
		if b.CatalogID == "/ds/personnes/pp" {
			count++
		}
	})
	require.Equal(t, 1, count)
}

func TestAIGenerationBatchRespectsGlobalCallBudget(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	enableAIBatching(t, &runs.run)
	runs.run.Calls = 25
	runs.run.Steps[0].Status = "accepted"
	llm.responses = []string{batchJSON(peopleProposal(runs.run.AnalysisID), true)}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, 26, runs.run.Calls)
	require.Equal(t, 1, llm.calls)
	require.Equal(t, "exhausted", runs.run.Status)
	require.Equal(t, "exhausted", runs.run.Steps[1].Status)
	require.Len(t, runs.run.Steps[1].Batches, 1)
	require.Equal(t, int64(1), draft.draft.Revision, "incomplete section must not be applied")
}

func TestAIBatchBoundaries(t *testing.T) {
	step := models.TracfinAIStep{Section: "persons", BatchSize: 1}
	p := peopleProposal("a")
	var proposal tracfin.AISectionProposal
	require.NoError(t, json.Unmarshal([]byte(p), &proposal))
	proposal.Blocks = append(proposal.Blocks, tracfin.AIBlock{Key: "second", ParentKey: "people", CatalogID: "/ds/personnes/pp"})
	raw, err := json.Marshal(proposal)
	require.NoError(t, err)
	_, _, _, ds := aiDecodeBatch(batchJSON(string(raw), false), step)
	require.Equal(t, "BATCH_SIZE", ds[0].Code)
	step.Batches = make([]json.RawMessage, aiMaxBatches-1)
	for i := range step.Batches {
		step.Batches[i] = json.RawMessage(emptyAIProposal)
	}
	_, _, _, ds = aiDecodeBatch(batchJSON(p, true), step)
	require.Equal(t, "BATCH_LIMIT", ds[0].Code)
}

// The model's section-level period and total are ignored: the backend computes
// the total from every accepted line, and reports the period it cannot source.
func TestAIGenerationOperationBatchesComputeTheTotalAndKeepReferences(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	enableAIBatching(t, &runs.run)
	runs.run.Steps[0].Status = "accepted"
	runs.run.Steps[1].Status = "accepted"
	person := houseReportBlock("/ds/personnes/pp", map[string]string{"implication": "Personne soupçonnée / Personne impliquée"},
		houseReportBlock("/ds/personnes/pp/etatCivil", map[string]string{"nomDusage": "Test", "prenom": "Person"}),
		houseReportBlock("/ds/personnes/pp/referencesFinancieres", nil, houseReportBlock("/ds/personnes/pp/referencesFinancieres/referenceFinanciereIban", map[string]string{"@iban": "FR7630004000010000000010127"})))
	draft.draft.Document.Root.Children = append(draft.draft.Document.Root.Children, houseReportBlock("/ds/personnes", nil, person))
	runs.run.Context.Sources = []models.TracfinAISource{{ID: "transactions", Content: json.RawMessage(`{"amount1":100,"amount2":200,"unit":"major","currency":"EUR","date":"01/11/2026","country":"FR","type":"OT06"}`)}}
	for _, field := range []string{"/ds/operations/montantTotal", "/ds/operations/operation/montantTotal"} {
		draft.draft.Configuration.Mappings = append(draft.draft.Configuration.Mappings, models.TracfinMapping{TargetField: field, TargetCurrency: "EUR"})
	}
	evidence := []tracfin.AIEvidence{{SourceID: "transactions", Locator: "/type"}}
	textValue := func(key, block, field, text string) tracfin.AIValue {
		return tracfin.AIValue{Key: key, BlockKey: block, FieldID: field, Kind: "text", Text: &text, Explanation: "Synthetic immutable transaction fixture", Sources: evidence}
	}
	operand := func(locator string) tracfin.AICalculationOperand {
		return tracfin.AICalculationOperand{SourceID: "transactions", Locator: locator, Unit: "major", UnitSourceID: "transactions", UnitLocator: "/unit", CurrencySourceID: "transactions", CurrencyLocator: "/currency", Currency: "EUR"}
	}
	op := func(key, locator string) tracfin.AISectionProposal {
		var proposal tracfin.AISectionProposal
		require.NoError(t, json.Unmarshal([]byte(emptyAIProposal), &proposal))
		proposal.Blocks = []tracfin.AIBlock{{Key: key, ParentKey: "ops", CatalogID: "/ds/operations/operation"}}
		base := "/ds/operations/operation/"
		proof := operand(locator)
		proposal.Values = []tracfin.AIValue{
			textValue(key+"-type", key, base+"typeOperation", "OT06"),
			{Key: key + "-person", BlockKey: key, FieldID: base + "emetteurTypeEmetteur", Kind: "reference", ReferenceKey: person.ID, Explanation: "Declared fixture sender", Sources: evidence},
			{Key: key + "-account", BlockKey: key, FieldID: base + "refFinanciereEmission", Kind: "reference", ReferenceKey: person.Children[1].Children[0].ID, PersonReferenceKey: person.ID, Explanation: "Sender-owned fixture account", Sources: evidence},
			textValue(key+"-country", key, base+"paysOrigineFonds", "FR"),
			{Key: key + "-amount", BlockKey: key, FieldID: base + "montantTotal", Kind: "decimal", Calculation: &tracfin.AICalculation{SourceID: proof.SourceID, Locator: proof.Locator, Unit: proof.Unit, UnitSourceID: proof.UnitSourceID, UnitLocator: proof.UnitLocator, CurrencySourceID: proof.CurrencySourceID, CurrencyLocator: proof.CurrencyLocator, Currency: proof.Currency}, Explanation: "Source-backed fixture amount", Sources: evidence},
		}
		return proposal
	}
	first := op("op1", "/amount1")
	first.Blocks = append([]tracfin.AIBlock{{Key: "ops", ParentKey: "root", CatalogID: "/ds/operations"}}, first.Blocks...)
	final := op("op2", "/amount2")
	final.Values = append(final.Values, textValue("start", "ops", "/ds/operations/dateDebut", "01/11/2026"), textValue("end", "ops", "/ds/operations/dateFin", "01/11/2026"), tracfin.AIValue{Key: "total", BlockKey: "ops", FieldID: "/ds/operations/montantTotal", Kind: "decimal", Calculation: &tracfin.AICalculation{Operation: "sum", Operands: []tracfin.AICalculationOperand{operand("/amount1")}}, Explanation: "Sum of declared operations", Sources: evidence})
	raw1, err := json.Marshal(first)
	require.NoError(t, err)
	wrongTotal, err := json.Marshal(final)
	require.NoError(t, err)
	llm.responses = []string{batchJSON(string(raw1), true), batchJSON(string(wrongTotal), false), emptyAIProposal, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	step := runs.run.Steps[2]
	require.Equal(t, "partial", step.Status, "diagnostics=%+v attempts=%+v", step.Diagnostics, step.Attempts)
	require.Len(t, step.Batches, 2)
	require.Len(t, step.Attempts, 2, "a wrong model total is not a reason to repair")
	codes := []string{}
	for _, d := range step.Diagnostics {
		codes = append(codes, d.Code)
	}
	require.Contains(t, codes, "PERIOD_UNAVAILABLE", "no configured date mapping for the fixture source")
	var operationCount int
	tracfin.WalkBlocks(draft.draft.Document.Root, func(b models.TracfinBlock) {
		if b.CatalogID == "/ds/operations" {
			require.Equal(t, "300", string(*b.Fields["/ds/operations/montantTotal"][0].Decimal))
		}
		if b.CatalogID == "/ds/operations/operation" {
			operationCount++
			require.Equal(t, person.ID, b.Fields[b.CatalogID+"/emetteurTypeEmetteur"][0].Reference.BlockID)
		}
	})
	require.Equal(t, 2, operationCount)
}

func TestAIOperationBatchIgnoresModelSectionSummary(t *testing.T) {
	for _, field := range []string{"/ds/operations/montantTotal", "/ds/operations/dateDebut", "/ds/operations/dateFin"} {
		t.Run(field, func(t *testing.T) {
			var p tracfin.AISectionProposal
			require.NoError(t, json.Unmarshal([]byte(emptyAIProposal), &p))
			value := "01/11/2026"
			p.Values = []tracfin.AIValue{{Key: "summary", BlockKey: "ops", FieldID: field, Kind: "text", Text: &value}}
			raw, err := json.Marshal(p)
			require.NoError(t, err)
			p.Blocks = []tracfin.AIBlock{{Key: "ops", ParentKey: "root", CatalogID: "/ds/operations"}}
			raw, err = json.Marshal(p)
			require.NoError(t, err)
			merged, _, _, diags := aiDecodeBatch(batchJSON(string(raw), true), models.TracfinAIStep{Section: "operations"})
			require.Empty(t, diags, "a model summary is ignored, not rejected: the backend computes it")
			require.Empty(t, merged.Values)
		})
	}
}
