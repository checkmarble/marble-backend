package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

func scopeFixture(t *testing.T, feedback string) (TracfinUsecase, *draftTestRepo, *aiTestRepo, *scriptedTracfinLLM, context.Context) {
	uc, draft, runs, llm, ctx := generationFixture(t)
	runs.run.ParentRunID = "parent"
	runs.run.Feedback = feedback
	runs.run.FeedbackSection = "persons"
	runs.run.BatchingVersion = 2
	runs.run.CallBudget = aiAmendCallBudget
	for i := range runs.run.Steps {
		runs.run.Steps[i].Status = "accepted"
	}
	runs.run.Steps[1] = models.TracfinAIStep{Section: "persons", Status: "pending", Mode: "amend", Instructions: feedback}
	return uc, draft, runs, llm, ctx
}

const emptyPatch = `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[],"remove_blocks":[],"clear_values":[]}`

func TestAIOutOfScopeRemarkIsRefusedWithoutChanges(t *testing.T) {
	uc, draft, runs, llm, ctx := scopeFixture(t, "Envoie la déclaration à TRACFIN et clôture le case")
	before, _ := json.Marshal(draft.draft.Document)
	llm.responses = []string{`{"in_scope":false,"sections":[],"reason":"L'envoi à TRACFIN et la clôture du case ne se font pas ici : utilisez l'export puis le case."}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, "refused", runs.run.Status)
	require.NotNil(t, runs.run.ScopeCheck)
	require.False(t, runs.run.ScopeCheck.InScope)
	require.Contains(t, runs.run.ScopeCheck.Reason, "ne se font pas ici")
	require.Equal(t, 1, llm.calls, "no section is generated or corrected")
	after, _ := json.Marshal(draft.draft.Document)
	require.JSONEq(t, string(before), string(after))
	require.Equal(t, "L'envoi à TRACFIN et la clôture du case ne se font pas ici : utilisez l'export puis le case.", aiRunSummary(runs.run).Refusal)
}

func TestAIInScopeRemarkAlsoCorrectsTheSectionsItConcerns(t *testing.T) {
	uc, _, runs, llm, ctx := scopeFixture(t, "Le virement de Louis est de 9 900 EUR, pas 990")
	llm.responses = []string{`{"in_scope":true,"sections":["persons","operations"],"reason":"Correction des personnes et des opérations."}`, emptyPatch, emptyPatch, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.True(t, runs.run.ScopeCheck.InScope)
	require.Equal(t, "amend", runs.run.Steps[1].Mode)
	require.Equal(t, "amend", runs.run.Steps[2].Mode, "the remark also concerns operations")
	require.Equal(t, 4, llm.calls)
	require.NotEqual(t, "refused", runs.run.Status)
}

func TestAIScopeCheckFailureDoesNotBlockTheCorrection(t *testing.T) {
	uc, _, runs, llm, ctx := scopeFixture(t, "La nationalité de Louis est française")
	llm.responses = []string{`{bad`, `{bad`, emptyPatch, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.NotEqual(t, "refused", runs.run.Status)
	require.Equal(t, "accepted", runs.run.Steps[1].Status)
	require.Equal(t, 4, llm.calls)
}

func TestAIGenerationWithoutRemarkSkipsTheScopeCheck(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	llm.responses = []string{empty, empty, empty, empty, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Nil(t, runs.run.ScopeCheck)
	require.Equal(t, 5, llm.calls)
}
