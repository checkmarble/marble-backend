package usecases

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/stretchr/testify/require"
)

// A rework corrects one section in place: no work-list call, no regeneration of
// the other sections, same block IDs, and the review sees the correction.
func TestAIAmendCorrectsInPlaceAndShowsTheChangeToTheReview(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	name, alias := "Petit", "Lou"
	civil := models.TracfinBlock{ID: "civil", CatalogID: "/ds/personnes/pp/etatCivil", Origin: &models.TracfinOrigin{Source: "ai"}, Fields: map[string][]models.TracfinValue{
		"/ds/personnes/pp/etatCivil/nomDusage": {{ID: "old", Kind: "text", Text: &name, Origin: &models.TracfinOrigin{Source: "ai", Resolution: "resolved"}}},
		"/ds/personnes/pp/etatCivil/alias":     {{ID: "alias", Kind: "text", Text: &alias, Origin: &models.TracfinOrigin{Source: "ai", Resolution: "resolved"}}},
	}}
	pp := models.TracfinBlock{ID: "pp", CatalogID: "/ds/personnes/pp", Origin: &models.TracfinOrigin{Source: "ai"}, Fields: map[string][]models.TracfinValue{}, Children: []models.TracfinBlock{civil}}
	draft.draft.Document.Root.Children = append(draft.draft.Document.Root.Children, models.TracfinBlock{ID: "people", CatalogID: "/ds/personnes", Origin: &models.TracfinOrigin{Source: "ai"}, Fields: map[string][]models.TracfinValue{}, Children: []models.TracfinBlock{pp}})
	runs.run.BatchingVersion = 2
	runs.run.CallBudget = aiAmendCallBudget
	runs.run.Context.Sources = []models.TracfinAISource{{ID: "feedback:run", Kind: "analyst_feedback", Content: json.RawMessage(`{"text":"Le nom d'usage est PETIT"}`)}}
	for i := range runs.run.Steps {
		runs.run.Steps[i].Status = "accepted"
	}
	runs.run.Steps[1] = models.TracfinAIStep{Section: "persons", Status: "pending", Mode: "amend", Instructions: "Le nom d'usage est PETIT"}
	upper := "PETIT"
	patch, _ := json.Marshal(tracfin.AIAmendProposal{
		AISectionProposal: tracfin.AISectionProposal{Blocks: []tracfin.AIBlock{}, Values: []tracfin.AIValue{{Key: "name", BlockKey: "civil", FieldID: "/ds/personnes/pp/etatCivil/nomDusage", Kind: "text", Text: &upper, Explanation: "Correction de l'analyste", Sources: []tracfin.AIEvidence{{SourceID: "feedback:run", Locator: "/text"}}}}, Attachments: []tracfin.AIAttachment{}, Missing: []string{}, Conflicts: []string{}},
		RemoveBlocks:      []string{}, ClearValues: []tracfin.AIValueRef{},
	})
	llm.responses = []string{string(patch), `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, 2, llm.calls, "one correction and one review, no work list and no other section")
	// partial only because the fixture person lacks its implication.
	require.Equal(t, "partial", runs.run.Steps[1].Status, "%+v", runs.run.Steps[1])
	require.Len(t, runs.run.Steps[1].Attempts, 1)
	var got *models.TracfinBlock
	tracfin.WalkBlocks(draft.draft.Document.Root, func(b models.TracfinBlock) {
		if b.ID == "civil" {
			got = &b
		}
	})
	require.NotNil(t, got, "the corrected block keeps its ID")
	require.Len(t, got.Fields["/ds/personnes/pp/etatCivil/nomDusage"], 1)
	require.Equal(t, "PETIT", *got.Fields["/ds/personnes/pp/etatCivil/nomDusage"][0].Text)
	require.Len(t, runs.run.Changes, 1)
	require.Equal(t, "analyst", runs.run.Changes[0].Origin)
	require.Contains(t, llm.prompts[1], "Le nom d'usage est PETIT", "the review receives the instruction and the patch")
}

func TestAIModelInputCarriesEarlierRemarks(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	runs.run.Remarks = []models.TracfinAIRemark{{RunID: "older", Section: "persons", Text: "Numéro fiscal 3320022"}}
	trace := tracfin.NewAITrace("test")
	defer trace.Close()
	_, err := uc.callAI(ctx, &runs.run, draft.draft, "persons", "", nil, nil, trace)
	require.NoError(t, err)
	require.Contains(t, llm.lastPrompt, `"analyst_remarks"`)
	require.Contains(t, llm.lastPrompt, "3320022")
}

func TestAIHistorySummarizesWithoutPromptsOrRawOutput(t *testing.T) {
	uc, _, runs, _, ctx := generationFixture(t)
	runs.run.ParentRunID = "parent"
	runs.run.Feedback = "Le nom d'usage est PETIT"
	runs.run.FeedbackSection = "persons"
	runs.run.Steps[1] = models.TracfinAIStep{Section: "persons", Status: "accepted", Mode: "amend", Instructions: runs.run.Feedback, Attempts: []models.TracfinAIAttempt{{Number: 1, Raw: "SECRET RAW OUTPUT"}}}
	runs.run.Changes = []models.TracfinAIChange{{Section: "persons", Origin: "analyst", Instructions: runs.run.Feedback}}
	history, err := uc.ListAIHistory(ctx, "case")
	require.NoError(t, err)
	require.Len(t, history, 1)
	h := history[0]
	require.Equal(t, "correction", h.Kind)
	require.Equal(t, "persons", h.FeedbackSection)
	require.Equal(t, "amend", h.Steps[1].Mode)
	require.Equal(t, 1, h.Steps[1].Attempts)
	require.Len(t, h.Changes, 1)
	raw, _ := json.Marshal(h)
	require.NotContains(t, string(raw), "SECRET RAW OUTPUT")
	require.NotContains(t, string(raw), "prompt")
}
