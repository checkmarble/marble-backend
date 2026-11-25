package engine

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	computedPath = "personnes.pp[pp-1].etatCivil.nomNaissance"
	inputPath1   = "personnes.pp[pp-2].etatCivil.nomNaissance"
	inputPath2   = "personnes.pp[pp-2].etatCivil.prenoms"
)

func computedFindingJob(t *testing.T, inputs ...core.Path) (*Orchestrator, *fakes) {
	t.Helper()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	reviewJob(t, f)
	f.Format.ComputedInputs = map[core.Path][]core.Path{computedPath: inputs}
	return o, f
}

func openTexts(t *testing.T, f *fakes) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, q := range questionsOf(t, f, models.SarQuestionStatusOpen) {
		out[q.Text] = q.Paths
	}
	return out
}

// Run 13: an open question on the transaction field, a finding on the computed
// line field: the finding is the same problem and is dropped.
func TestReview_FindingOnComputedFieldCoveredByAnOpenQuestionIsDropped(t *testing.T) {
	ctx := context.Background()
	o, f := computedFindingJob(t, inputPath1)
	require.NoError(t, f.Repo.CreateSarQuestions(ctx, nil, []models.SarQuestion{{
		Id: uuid.New(), DeclarationId: f.DeclId, Kind: models.SarQuestionKindQuestion,
		Paths: []string{inputPath1}, Text: "valeur manquante", Status: models.SarQuestionStatusOpen,
	}}))
	f.Runner.On(core.TaskKindReview, tgtPP1, Step{Out: core.TaskOutput{Findings: []core.ReviewFinding{
		{Paths: []core.Path{computedPath}, Severity: "important", Message: "ligne sans référence"},
	}}})
	require.NoError(t, o.RunJob(ctx, f.JobId))
	assert.Equal(t, map[string][]string{"valeur manquante": {inputPath1}}, openTexts(t, f))
}

// Without an open question the finding is stored on the input fields.
func TestReview_FindingOnComputedFieldIsStoredOnItsInputs(t *testing.T) {
	ctx := context.Background()
	o, f := computedFindingJob(t, inputPath1, inputPath2)
	f.Runner.On(core.TaskKindReview, tgtPP1, Step{Out: core.TaskOutput{Findings: []core.ReviewFinding{
		{Paths: []core.Path{computedPath, inputPath1}, Severity: "important", Message: "ligne sans référence"},
	}}})
	require.NoError(t, o.RunJob(ctx, f.JobId))
	assert.Equal(t, map[string][]string{"ligne sans référence": {inputPath1, inputPath2}}, openTexts(t, f),
		"both inputs, once each, in a stable order")
}

// A computed path the format does not know stays as it is.
func TestReview_FindingOnUnknownComputedPathIsUnchanged(t *testing.T) {
	ctx := context.Background()
	o, f := computedFindingJob(t) // no inputs
	f.Runner.On(core.TaskKindReview, tgtPP1, Step{Out: core.TaskOutput{Findings: []core.ReviewFinding{
		{Paths: []core.Path{computedPath}, Severity: "important", Message: "total"},
	}}})
	require.NoError(t, o.RunJob(ctx, f.JobId))
	assert.Equal(t, map[string][]string{"total": {computedPath}}, openTexts(t, f))
}

// A contesting finding on a computed path covers the paths of the question once
// mapped.
func TestCheckReview_ContestingFindingOnComputedPathIsMappedBeforeTheCoverCheck(t *testing.T) {
	c, tc, qid := reviewChecker(t)
	tc.Format = &fakeFormat{Format: tc.Format, ComputedInputs: map[core.Path][]core.Path{computedPath: {nomPP1}}}
	c.tc = tc
	findings, errs := c.checkReview(reviewJSON(computedPath, "Dupont", "Dupont", qid.String(), computedPath))
	// The option writes the input field: allowed since the finding is about it.
	require.Empty(t, errs)
	require.Len(t, findings, 1)
	assert.Equal(t, []core.Path{nomPP1}, findings[0].Paths)
}
