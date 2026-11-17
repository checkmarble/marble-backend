package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// The final apply checks the sources like the agent's dry run: a fill citing an
// object outside the allowed set is rejected, and an option fill failing the
// same check is dropped (the option stays a label) and recorded as rejected.
func TestRunJob_FillAndOptionSourcesOutsideScopeRejected(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)

	other := core.SourceRef{Object: core.ObjectRef{Table: "t", ObjectID: "other"}, Field: "f", RawValue: "v"}
	bad := Fill(pathNomPP1, "DUPONT")
	bad.Sources = []core.SourceRef{other}
	badOpt := Fill(pathImplPP1, "IMP1")
	badOpt.Sources = []core.SourceRef{other}
	goodOpt := Fill(pathImplPP1, "IMP2")

	a := Spec("a", tgtPP1)
	f.Format.Waves = [][]core.TaskSpec{{a}}
	f.Runner.On("a", tgtPP1, Step{Out: core.TaskOutput{
		Fills: []core.Fill{bad},
		Questions: []core.QuestionDraft{{
			Paths: []core.Path{core.Path(pathImplPP1)},
			Text:  "Implication ?",
			Options: []core.QuestionOption{
				{Label: "A", Fills: []core.Fill{badOpt}},
				{Label: "B", Fills: []core.Fill{goodOpt}},
			},
		}},
	}})

	require.NoError(t, o.RunJob(ctx, f.JobId))

	_, ok := valueOf(t, f, pathNomPP1)
	require.False(t, ok, "a fill citing an object outside the allowed sources must not be applied")

	qs := questionsOf(t, f, "")
	require.Len(t, qs, 1)
	var opts []core.QuestionOption
	require.NoError(t, json.Unmarshal(qs[0].Options, &opts))
	require.Len(t, opts, 2)
	require.Empty(t, opts[0].Fills, "the option with a foreign source stays a plain label")
	require.Len(t, opts[1].Fills, 1)

	var res taskResult
	require.NoError(t, json.Unmarshal(f.Repo.Task(t, "a", tgtPP1).Output, &res))
	var codes []string
	for _, r := range res.Rejected {
		codes = append(codes, string(r.Path)+"|"+r.Code)
	}
	require.ElementsMatch(t, []string{
		pathNomPP1 + "|" + core.CodeSourceOutOfScope,
		pathImplPP1 + "|" + core.CodeSourceOutOfScope,
	}, codes)
}
