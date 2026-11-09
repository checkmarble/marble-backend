package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

// A worker shutdown cancels the in-flight model call: this is not a model
// failure, consumes no attempt and leaves the run resumable.
func TestAIGenerationWorkerShutdownReleasesTheAttempt(t *testing.T) {
	uc, _, runs, llm, base := generationFixture(t)
	ctx, cancel := context.WithCancel(base)
	llm.before = func(call int) {
		if call == 0 {
			cancel()
		}
	}
	llm.errs = map[int]error{0: context.Canceled}
	err := uc.GenerateAI(ctx, "case", "run")
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, "running", runs.run.Status)
	require.Empty(t, runs.run.Steps[0].Attempts)
	require.Zero(t, runs.run.Calls)

	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	llm.errs = nil
	llm.before = nil
	llm.calls = 0
	llm.responses = []string{empty, empty, empty, empty, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(base, "case", "run"))
	require.Equal(t, "accepted", runs.run.Steps[0].Status)
	require.Len(t, runs.run.Steps[0].Attempts, 1)
}

func TestAIGenerationTransientAccessCheckIsNotAnAuthorizationFailure(t *testing.T) {
	uc, _, runs, llm, ctx := generationFixture(t)
	uc.ai.CheckActor = func(context.Context, string, string) error { return context.Canceled }
	require.ErrorIs(t, uc.GenerateAI(ctx, "case", "run"), context.Canceled)
	require.NotEqual(t, "authorization_failed", runs.run.Status)
	require.Zero(t, llm.calls)
}

func TestAIGenerationModelErrorsAreNamed(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{errors.Mark(errors.New("Error 429, Message: Resource exhausted"), models.LLMRateLimitedError), "rate limit"},
		{context.DeadlineExceeded, "timed out"},
		{errors.New("Error 400, Message: model gemini-x not found"), "gemini-x not found"},
	} {
		uc, _, runs, llm, ctx := generationFixture(t)
		empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
		llm.responses = []string{"", empty, empty, empty, empty, `{"findings":[]}`}
		llm.errs = map[int]error{0: tc.err}
		require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
		at := runs.run.Steps[0].Attempts[0]
		require.Equal(t, "MODEL_UNAVAILABLE", at.Diagnostics[0].Code)
		require.Contains(t, at.Diagnostics[0].Message, tc.want)
	}
}
