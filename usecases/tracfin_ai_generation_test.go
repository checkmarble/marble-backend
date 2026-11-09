package usecases

import (
	"context"
	"encoding/json"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/ai_agent"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

type scriptedTracfinLLM struct {
	prompts       []string
	finishReasons map[int]string
	lastPrompt    string
	calls         int
	responses     []string
	before        func(int)
	errs          map[int]error
}

func (l *scriptedTracfinLLM) TracfinAIPrompts() (map[string]string, error) { return nil, nil }
func (l *scriptedTracfinLLM) TracfinAIModel() (string, string, error)      { return "main", "test", nil }
func (l *scriptedTracfinLLM) TracfinAICompleteWithModel(_ context.Context, _ string, prompt string, _ jsonschema.Schema, _, _ string) (ai_agent.TracfinAIResponse, error) {
	l.lastPrompt = prompt
	l.prompts = append(l.prompts, prompt)
	if l.before != nil {
		l.before(l.calls)
	}
	text := `{"findings":[]}`
	if l.calls < len(l.responses) {
		text = l.responses[l.calls]
	}
	finish := "stop"
	if reason := l.finishReasons[l.calls]; reason != "" {
		finish = reason
	}
	if err := l.errs[l.calls]; err != nil {
		l.calls++
		return ai_agent.TracfinAIResponse{}, err
	}
	l.calls++
	return ai_agent.TracfinAIResponse{Text: text, FinishReason: finish}, nil
}
func generationFixture(t *testing.T) (TracfinUsecase, *draftTestRepo, *aiTestRepo, *scriptedTracfinLLM, context.Context) {
	t.Helper()
	org := uuid.New()
	c, e := tracfin.LoadCatalog()
	require.NoError(t, e)
	d := models.TracfinDraft{CaseID: "case", OrgID: org.String(), Revision: 1, Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "root", CatalogID: "/ds"}}}
	putAIAnalysis(&d, "Analyse humaine inchangée\n& <exact>", "actor", "")
	repo := &draftTestRepo{draft: d, exists: true}
	runs := &aiTestRepo{run: models.TracfinAIRun{ID: "run", OrgID: org.String(), CaseID: "case", ActorID: "actor", Revision: 1, DraftRevision: 1, Status: "queued", CatalogHash: c.Hash, AnalysisID: d.AI.CurrentAnalysisID, Context: models.TracfinAIContext{Complete: true}, PromptSnapshot: map[string]string{"tracfin/system.md": "system", "tracfin/sections/persons.md": "{{.Input}}", "tracfin/sections/attachments.md": "{{.Input}}", "tracfin/sections/operations.md": "{{.Input}}", "tracfin/sections/declaration.md": "{{.Input}}", "tracfin/repair_section.md": "{{.Input}}", "tracfin/review.md": "{{.Input}}"}}}
	for _, s := range tracfin.AISections {
		runs.run.Steps = append(runs.run.Steps, models.TracfinAIStep{Section: s, Status: "pending"})
	}
	llm := &scriptedTracfinLLM{}
	uc := aiTestUsecase(org, repo, runs)
	uc.ai.LLM = llm
	return uc, repo, runs, llm, draftTestContext(org)
}
func TestAIGenerationMalformedRepairIsBoundedAndReplayDoesNotDuplicate(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	llm.responses = []string{`{bad`, empty, empty, empty, empty, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Len(t, runs.run.Steps[0].Attempts, 2)
	require.Equal(t, "JSON_INVALID", runs.run.Steps[0].Attempts[0].Diagnostics[0].Code)
	original := draft.draft.AI.Analyses[0].Text
	require.Equal(t, original, *draft.draft.Document.Root.Fields["/ds/analyse"][0].Text)
	require.Equal(t, 6, llm.calls)
	revision := draft.draft.Revision
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, 6, llm.calls)
	require.Equal(t, revision, draft.draft.Revision)
}
func TestAIGenerationExhaustsThreeAttemptsAndPreservesCandidate(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	llm.responses = []string{`{bad`, `{bad`, `{bad`, `{bad`, `{bad`, `{bad`, `{bad`, `{bad`, `{bad`, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Len(t, runs.run.Steps[0].Attempts, 3)
	require.Equal(t, "exhausted", runs.run.Steps[0].Status)
	require.Equal(t, "blocked", runs.run.Steps[2].Status)
	require.LessOrEqual(t, llm.calls, 10)
}
func TestAIGenerationDoesNotApplyOverConcurrentHumanEdit(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	llm.responses = []string{`{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`}
	llm.before = func(int) { draft.draft.Revision = 2 }
	require.ErrorIs(t, uc.GenerateAI(ctx, "case", "run"), models.ConflictError)
	require.Equal(t, "stale", runs.run.Status)
	require.Equal(t, int64(2), draft.draft.Revision)
}
func TestAIGenerationPersistsEvidenceWithDebugOff(t *testing.T) {
	t.Setenv("TRACFIN_AI_DEBUG", "false")
	uc, draft, runs, llm, ctx := generationFixture(t)
	c, _ := tracfin.LoadCatalog()
	_ = c
	// Empty proposals still give all preexisting/manual values deterministic evidence.
	raw, _ := json.Marshal(tracfin.AISectionProposal{Blocks: []tracfin.AIBlock{}, Values: []tracfin.AIValue{}, Attachments: []tracfin.AIAttachment{}, Missing: []string{}, Conflicts: []string{}})
	llm.responses = []string{string(raw), string(raw), string(raw), string(raw), `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	v := draft.draft.Document.Root.Fields["/ds/analyse"][0]
	require.NotEmpty(t, v.Origin.Explanation)
	require.NotEmpty(t, v.Origin.Evidence)
	require.NotEqual(t, "completed", runs.run.Status)
}

func TestAIGenerationGlobalCorrectionRunsOnceAndPreservesReviewHistory(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	finding := tracfin.AIReview{Findings: []tracfin.AIReviewFinding{{ID: "f1", Section: "persons", Message: "Recheck identity against analyst text", Blocking: true, Sources: []tracfin.AIEvidence{{SourceID: "analysis:" + runs.run.AnalysisID, Locator: "text"}}}}}
	raw, _ := json.Marshal(finding)
	emptyPatch := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[],"remove_blocks":[],"clear_values":[]}`
	llm.responses = []string{empty, empty, empty, empty, string(raw), emptyPatch, string(raw)}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, 1, runs.run.ReviewCycle, "status=%s calls=%d steps=%+v review=%s", runs.run.Status, llm.calls, runs.run.Steps, string(runs.run.Review))
	require.Len(t, runs.run.PreviousSteps, 1)
	require.Len(t, runs.run.ReviewAttempts, 2)
	// Only the section named by the blocking finding is corrected, in place.
	require.Equal(t, 7, llm.calls)
	require.Equal(t, "amend", runs.run.Steps[1].Mode)
	require.Contains(t, runs.run.Steps[1].Instructions, "Recheck identity against analyst text")
	require.Contains(t, llm.prompts[5], "Recheck identity against analyst text")
	for _, i := range []int{0, 2, 3} {
		require.Empty(t, runs.run.Steps[i].Mode, "untouched sections keep their accepted content")
		require.Len(t, runs.run.Steps[i].Attempts, 1)
	}
	require.Len(t, runs.run.Changes, 1)
	require.Equal(t, "review", runs.run.Changes[0].Origin)
	require.Contains(t, llm.prompts[6], `"changes"`, "the second review sees the correction")
	require.Equal(t, "needs_input", runs.run.Status)
	require.Equal(t, draft.draft.AI.Analyses[0].Text, *draft.draft.Document.Root.Fields["/ds/analyse"][0].Text)
}
func TestAIGenerationRevokedPermissionReleasesActiveSlot(t *testing.T) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	uc.ai.CheckActor = func(context.Context, string, string) error { return models.ForbiddenError }
	require.ErrorIs(t, uc.GenerateAI(ctx, "case", "run"), models.ForbiddenError)
	require.Equal(t, "authorization_failed", runs.run.Status)
	require.Equal(t, int64(1), draft.draft.Revision)
	require.Zero(t, llm.calls)
}
func TestAIGenerationContextBudgetStopsWithoutModelCalls(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	runs.run.Context.Sources = []models.TracfinAISource{{ID: "s", Content: json.RawMessage(`"` + strings.Repeat("x", 600<<10) + `"`)}}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, "context_budget", runs.run.Status)
	require.Equal(t, "CONTEXT_BUDGET", runs.run.Steps[0].Diagnostics[0].Code)
	require.Zero(t, llm.calls)
	require.Zero(t, runs.run.Calls)
}

func TestAIRegenerationPreservesManuallyAmendedScalarAndPrunesAutomaticBlocks(t *testing.T) {
	s := "FR"
	manual := "1"
	root := models.TracfinBlock{ID: "root", CatalogID: "/ds", Children: []models.TracfinBlock{
		{ID: "civil", CatalogID: "/ds/personnes/pp/etatCivil", Origin: &models.TracfinOrigin{Source: "ai"}, Fields: map[string][]models.TracfinValue{
			"/ds/personnes/pp/etatCivil/nationalite":       {{ID: "nationality", Kind: "text", Text: &s, Origin: &models.TracfinOrigin{Source: "ai"}}},
			"/ds/personnes/pp/etatCivil/nationalite/@type": {{ID: "attribute", ParentValueID: "nationality", Kind: "text", Text: &manual, Origin: &models.TracfinOrigin{Source: "manual"}}}}},
		{ID: "obsolete", CatalogID: "/ds/personnes/pp", Origin: &models.TracfinOrigin{Source: "ai"}}}}
	aiRemoveGenerated(&root, "persons")
	require.Len(t, root.Children, 1)
	v := root.Children[0].Fields["/ds/personnes/pp/etatCivil/nationalite"]
	require.Len(t, v, 1)
	require.Equal(t, "nationality", v[0].ID)
	require.Equal(t, "manual", v[0].Origin.Source)
	require.Equal(t, "nationality", root.Children[0].Fields["/ds/personnes/pp/etatCivil/nationalite/@type"][0].ParentValueID)
}
func TestAIRegenerationPreservesManualIncomingReferencesAndDropsAutomaticAttachments(t *testing.T) {
	root := models.TracfinBlock{ID: "root", CatalogID: "/ds", Children: []models.TracfinBlock{
		{ID: "person", CatalogID: "/ds/personnes/pp", Origin: &models.TracfinOrigin{Source: "ai"}},
		{ID: "op", CatalogID: "/ds/operations/operation", Fields: map[string][]models.TracfinValue{"/ds/operations/operation/emetteurTypeEmetteur": {{ID: "ref", Kind: "reference", Reference: &models.TracfinReference{BlockID: "person"}, Origin: &models.TracfinOrigin{Source: "manual"}}}}},
		{ID: "piece", CatalogID: "/ds/pieceJointe/piece", Origin: &models.TracfinOrigin{Source: "ai"}, Attachment: &models.TracfinAttachment{SourceFileID: "old"}}}}
	aiRemoveGenerated(&root, "persons")
	require.Equal(t, "person", root.Children[0].ID)
	aiRemoveGenerated(&root, "attachments")
	for _, b := range root.Children {
		require.NotEqual(t, "piece", b.ID)
	}
}

func TestAIRepairReceivesFrozenSectionInstructions(t *testing.T) {
	t.Setenv("TRACFIN_AI_DEBUG", "false")
	uc, draft, runs, llm, ctx := generationFixture(t)
	runs.run.PromptSnapshot["tracfin/sections/persons.md"] = "Original frozen identity instructions.\n{{.Input}}"
	trace := tracfin.NewAITrace("test")
	defer trace.Close()
	_, err := uc.callAI(ctx, &runs.run, draft.draft, "persons", "{bad", nil, nil, trace)
	require.NoError(t, err)
	var input map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(llm.lastPrompt), &input))
	require.Contains(t, string(input["section_instructions"]), "Original frozen identity instructions")
}
