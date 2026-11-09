package usecases

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/stretchr/testify/require"
)

func TestAIGenerationOperationsRunAfterPartialPersons(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	partial := `{"blocks":[],"values":[],"attachments":[],"missing":["date de naissance inconnue"],"conflicts":[]}`
	llm.responses = []string{empty, partial, empty, empty, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, "partial", runs.run.Steps[1].Status)
	require.Equal(t, "accepted", runs.run.Steps[2].Status, "%+v", runs.run.Steps[2])
	require.Equal(t, 5, llm.calls)
}

func TestAIGenerationRepairsInvalidReviewEvidence(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	invalid, _ := json.Marshal(tracfin.AIReview{Findings: []tracfin.AIReviewFinding{{ID: "f1", Section: "persons", Message: "Check identity", Sources: []tracfin.AIEvidence{{SourceID: "configuration", Locator: "settings.root.children.1"}}}}})
	llm.responses = []string{empty, empty, empty, empty, string(invalid), `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.NotEqual(t, "review_failed", runs.run.Status, string(runs.run.Review))
	require.Len(t, runs.run.ReviewAttempts, 2)
	require.Equal(t, "rejected", runs.run.ReviewAttempts[0].Status)
	require.NotEmpty(t, runs.run.ReviewAttempts[0].Diagnostics)
	require.Equal(t, "accepted", runs.run.ReviewAttempts[1].Status)
	require.True(t, strings.Contains(llm.prompts[5], "REVIEW_EVIDENCE_INVALID"), "repair prompt must carry the diagnostics")
}

func TestAIGenerationReviewFailsAfterThreeInvalidAttempts(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	llm.responses = []string{empty, empty, empty, empty, `{bad`, `{bad`, `{bad`, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, "review_failed", runs.run.Status)
	require.Len(t, runs.run.ReviewAttempts, 3)
	require.Equal(t, 7, llm.calls)
}

func TestAIPromptCatalogOmitsCalculatedFields(t *testing.T) {
	c, err := tracfin.LoadCatalog()
	require.NoError(t, err)
	fields := aiPromptCatalog(c, "persons").(map[string]any)["fields"].(map[string]tracfin.AIFieldSpec)
	require.NotContains(t, fields, "/ds/personnes/pp/id")
	require.Contains(t, fields, "/ds/personnes/pp/etatCivil/prenom")
}
