package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type aiTestRepo struct {
	run     models.TracfinAIRun
	remarks []models.TracfinAIRemark
}

func (r *aiTestRepo) ListTracfinAIRuns(_ context.Context, _ repositories.Executor, org, caseID string, _ int) ([]models.TracfinAIRun, error) {
	if r.run.OrgID != org || r.run.CaseID != caseID {
		return nil, nil
	}
	return []models.TracfinAIRun{r.run}, nil
}
func (r *aiTestRepo) ListTracfinAIRemarks(_ context.Context, _ repositories.Executor, org, caseID string) ([]models.TracfinAIRemark, error) {
	return r.remarks, nil
}

func (r *aiTestRepo) CreateTracfinAIRun(_ context.Context, _ repositories.Executor, run models.TracfinAIRun) error {
	r.run = run
	return nil
}
func (r *aiTestRepo) GetTracfinAIRun(_ context.Context, _ repositories.Executor, org, caseID, id string) (models.TracfinAIRun, error) {
	if r.run.OrgID != org || r.run.CaseID != caseID || r.run.ID != id {
		return models.TracfinAIRun{}, models.NotFoundError
	}
	return r.run, nil
}
func (r *aiTestRepo) SaveTracfinAIRun(_ context.Context, _ repositories.Executor, run models.TracfinAIRun, expected int64) error {
	if r.run.Revision != expected {
		return models.ConflictError
	}
	run.Revision++
	r.run = run
	return nil
}
func aiTestUsecase(org uuid.UUID, draft *draftTestRepo, runs *aiTestRepo) TracfinUsecase {
	uc := NewTracfinUsecase(org, draft, tracfinCases{models.Case{Id: "case", OrganizationId: org, Outcome: models.CaseConfirmedRisk, Type: models.CaseTypeDecision}}, nil, nil, tracfinExec{}, TracfinDraftDependencies{Repository: draft, Transactions: draftTestTransactions{}})
	uc.ai = &TracfinAIDependencies{Repository: runs, Enqueue: func(context.Context, repositories.Transaction, models.TracfinAIArgs) error { return nil }}
	return uc
}

func TestAIAnalysisPreservesExactTextAndRejectsOversize(t *testing.T) {
	text := "  Analyse\nligne é & < >\tfin  "
	require.NoError(t, validateAIAnalysis(text))
	require.NoError(t, validateAIAnalysis(strings.Repeat("é", 23000)))
	require.Error(t, validateAIAnalysis(strings.Repeat("é", 23001)))
	d := models.TracfinDraft{}
	putAIAnalysis(&d, text, "actor", "")
	got, err := currentAIAnalysis(d)
	require.NoError(t, err)
	require.Equal(t, text, got.Text)
	require.Equal(t, text, *d.Document.Root.Fields["/ds/analyse"][0].Text)
	require.Equal(t, "actor", d.Document.Root.Fields["/ds/analyse"][0].Origin.ActorID)
	changed := "human edit"
	d.Document.Root.Fields["/ds/analyse"][0].Text = &changed
	_, err = currentAIAnalysis(d)
	require.Error(t, err)
}

func TestAICheckpointStaleDraftPreservesHumanEdit(t *testing.T) {
	org := uuid.New()
	human := "human edit"
	draft := &draftTestRepo{exists: true, draft: models.TracfinDraft{Revision: 2, Document: models.TracfinDocument{Root: models.TracfinBlock{Fields: map[string][]models.TracfinValue{"/ds/analyse": {{Text: &human}}}}}}}
	runs := &aiTestRepo{run: models.TracfinAIRun{ID: "run", OrgID: org.String(), CaseID: "case", Revision: 1, DraftRevision: 1, Status: "running"}}
	uc := aiTestUsecase(org, draft, runs)
	generated := "AI overwrite"
	proposed := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{Fields: map[string][]models.TracfinValue{"/ds/analyse": {{Text: &generated}}}}}}
	run := runs.run
	err := uc.checkpointAI(draftTestContext(org), &run, &proposed)
	require.ErrorIs(t, err, models.ConflictError)
	require.Equal(t, "stale", runs.run.Status)
	require.Equal(t, int64(2), draft.draft.Revision)
	require.Equal(t, human, *draft.draft.Document.Root.Fields["/ds/analyse"][0].Text)
}

func TestAIResumePreservesAttemptAndCallBudget(t *testing.T) {
	org := uuid.New()
	draft := &draftTestRepo{exists: true, draft: models.TracfinDraft{Revision: 4}}
	runs := &aiTestRepo{run: models.TracfinAIRun{ID: "run", OrgID: org.String(), CaseID: "case", Revision: 2, DraftRevision: 4, Status: "failed", Calls: 7, ReviewCycle: 1, Steps: []models.TracfinAIStep{{Section: "persons", Attempts: []models.TracfinAIAttempt{{Number: 1}, {Number: 2}}}}}}
	uc := aiTestUsecase(org, draft, runs)
	got, err := uc.ResumeAI(draftTestContext(org), "case", "run", 4)
	require.NoError(t, err)
	require.Equal(t, "queued", got.Status)
	require.Equal(t, 7, runs.run.Calls)
	require.Equal(t, 1, runs.run.ReviewCycle)
	require.Len(t, runs.run.Steps[0].Attempts, 2)
}

func TestAIApprovalRejectsMissingValidationLayers(t *testing.T) {
	org := uuid.New()
	draft := &draftTestRepo{exists: true, draft: models.TracfinDraft{Revision: 4, AI: &models.TracfinAIMetadata{RunID: "run"}}}
	runs := &aiTestRepo{run: models.TracfinAIRun{ID: "run", OrgID: org.String(), CaseID: "case", Revision: 1, DraftRevision: 4, Status: "completed", Validation: &models.TracfinValidation{Layers: map[string]models.TracfinValidationLayer{"official_xsd": {Status: "unavailable"}}}}}
	uc := aiTestUsecase(org, draft, runs)
	_, err := uc.ApproveAI(draftTestContext(org), "case", "run", 4)
	require.Error(t, err)
	require.Zero(t, draft.draft.AI.ReviewedRevision)
	require.Equal(t, int64(4), draft.draft.Revision)
}

func (r *aiTestRepo) InvalidateTracfinAIActive(_ context.Context, _ repositories.Executor, org, caseID string) error {
	if r.run.OrgID == org && r.run.CaseID == caseID && (r.run.Status == "queued" || r.run.Status == "running") {
		r.run.Status = "stale"
		r.run.Revision++
	}
	return nil
}

type reworkTestRepo struct {
	*aiTestRepo
	parent models.TracfinAIRun
}

func (r *reworkTestRepo) GetTracfinAIRun(ctx context.Context, exec repositories.Executor, org, caseID, id string) (models.TracfinAIRun, error) {
	if id == r.parent.ID && org == r.parent.OrgID && caseID == r.parent.CaseID {
		return r.parent, nil
	}
	return r.aiTestRepo.GetTracfinAIRun(ctx, exec, org, caseID, id)
}

type reworkTestLLM struct{ scriptedTracfinLLM }

func (*reworkTestLLM) TracfinAIPrompts() (map[string]string, error) {
	prompts := map[string]string{}
	for _, path := range models.AiAgentTracfinPromptPaths {
		prompts[path] = "{{.Input}}"
	}
	return prompts, nil
}
func TestAIReworkCorrectsOnlyTheRequestedSection(t *testing.T) {
	dm, tables := task8PersonCardModel()
	w := task8NewWorld(dm, tables, nil, nil)
	c, _ := w.uc.cases.GetCase(w.ctx, "case")
	c.Type = models.CaseTypeDecision
	w.uc.cases = task8Cases{c, nil}
	w.repo.config = models.TracfinConfiguration{OrgID: w.org.String()}
	w.repo.configErr = nil
	_, err := w.uc.CreateDraft(w.ctx, "case", false)
	require.NoError(t, err)
	putAIAnalysis(&w.repo.draft, "Exact human analysis", "actor", "")
	analysisID := w.repo.draft.AI.CurrentAnalysisID
	parent := models.TracfinAIRun{ID: "original", OrgID: w.org.String(), CaseID: "case", Revision: 1, DraftRevision: 1, Status: "needs_input", Calls: 7}
	for _, section := range tracfin.AISections {
		parent.Steps = append(parent.Steps, models.TracfinAIStep{Section: section, Status: "accepted"})
	}
	w.uc.WithAICollector(TracfinAICollectorDependencies{Repository: &aiCollectorRepo{}, Files: aiNoFiles{}, Blobs: w.blobs})
	sourceContext, e := w.uc.CollectAIContext(w.ctx, "case")
	require.NoError(t, e)
	parent.Context = sourceContext
	parent.AnalysisID = analysisID
	w.repo.draft.AI.RunID = parent.ID
	runs := &reworkTestRepo{aiTestRepo: &aiTestRepo{}, parent: parent}
	queued := false
	w.uc.ai = &TracfinAIDependencies{Repository: runs, LLM: &reworkTestLLM{}, Enqueue: func(context.Context, repositories.Transaction, models.TracfinAIArgs) error { queued = true; return nil }}
	w.uc.WithAICollector(TracfinAICollectorDependencies{Repository: &aiCollectorRepo{}, Files: aiNoFiles{}, Blobs: w.blobs})
	// An earlier remark, made on another run of the case, keeps applying.
	runs.remarks = []models.TracfinAIRemark{{RunID: "older", Section: "persons", Text: "Numéro fiscal 3320022"}}
	run, err := w.uc.ReworkAI(w.ctx, "case", "original", models.TracfinAIInput{ExpectedRevision: 1, Section: "operations", Remarks: "Check the amount against the newly explained facts"})
	require.NoError(t, err)
	require.Equal(t, runs.remarks, run.Remarks)
	olderSource := false
	for _, source := range run.Context.Sources {
		if source.ID == "feedback:older" {
			olderSource = true
			require.Contains(t, string(source.Content), "3320022")
		}
	}
	require.True(t, olderSource, "an earlier remark stays citable as evidence")
	require.True(t, queued)
	require.NotEqual(t, parent.ID, run.ID)
	require.Equal(t, parent.ID, run.ParentRunID)
	require.Equal(t, "accepted", run.Steps[0].Status)
	require.Equal(t, "accepted", run.Steps[1].Status)
	require.Equal(t, "pending", run.Steps[2].Status)
	require.Equal(t, "amend", run.Steps[2].Mode, "only the requested section is corrected, in place")
	require.Equal(t, "Check the amount against the newly explained facts", run.Steps[2].Instructions)
	require.Equal(t, "accepted", run.Steps[3].Status, "later sections keep their accepted content")
	require.Empty(t, run.Steps[3].Mode)
	require.Equal(t, analysisID, run.AnalysisID)
	require.Equal(t, 7, runs.parent.Calls)
	found := false
	for _, source := range run.Context.Sources {
		if source.ID == "feedback:"+run.ID {
			found = true
		}
	}
	require.True(t, found)
	// A correction keeps block IDs, so people referencing pieces stay valid; a removed
	// piece detaches its references, which validation and the review then report.
	oldPiece := models.TracfinBlock{ID: "old-piece", CatalogID: "/ds/pieceJointe/piece", Origin: &models.TracfinOrigin{Source: "ai"}}
	oldPerson := models.TracfinBlock{ID: "old-person", CatalogID: "/ds/personnes/pp/etatCivil", Origin: &models.TracfinOrigin{Source: "ai"}, Fields: map[string][]models.TracfinValue{"/ds/personnes/pp/etatCivil/idPieceJointe": {{ID: "old-reference", Kind: "reference", Reference: &models.TracfinReference{BlockID: oldPiece.ID}, Origin: &models.TracfinOrigin{Source: "ai"}}}}}
	w.repo.draft.Document.Root.Children = append(w.repo.draft.Document.Root.Children, oldPiece, oldPerson)
	attachmentRework, e := w.uc.ReworkAI(w.ctx, "case", "original", models.TracfinAIInput{ExpectedRevision: 1, Section: "attachments", Remarks: "Replace the identity evidence file"})
	require.NoError(t, e)
	require.Equal(t, "attachments", attachmentRework.Steps[0].Section)
	require.Equal(t, "pending", attachmentRework.Steps[0].Status)
	require.Equal(t, "amend", attachmentRework.Steps[0].Mode)
	require.Equal(t, "persons", attachmentRework.Steps[1].Section)
	require.Equal(t, "accepted", attachmentRework.Steps[1].Status)
	putAIAnalysis(&w.repo.draft, "New human analysis with changed identities", "actor", "")
	changed, e := w.uc.ReworkAI(w.ctx, "case", "original", models.TracfinAIInput{ExpectedRevision: 1, Section: "operations", Remarks: "Recheck operations"})
	require.NoError(t, e)
	require.Equal(t, "attachments", changed.FromSection)
	require.Equal(t, "pending", changed.Steps[0].Status)
	for _, step := range changed.Steps {
		require.Empty(t, step.Mode, "a changed analysis regenerates every section")
	}
	require.NotEmpty(t, changed.ReworkReason)
}

func TestAISectionsRespectPersonAttachmentDependencies(t *testing.T) {
	require.Equal(t, []string{"attachments", "persons", "operations", "declaration"}, tracfin.AISections)
}
