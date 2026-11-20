package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const itemPP1 = core.Path("personnes.pp[pp-1]")

func reviewCheckerOn(t *testing.T, contested ...core.Path) (*checker, uuid.UUID) {
	t.Helper()
	c, tc, qid := reviewChecker(t)
	tc.OpenQuestions[0].Paths = contested
	return c, qid
}

func findingJSON(paths []string, optionsJSON, contests string) json.RawMessage {
	quoted, _ := json.Marshal(paths)
	return json.RawMessage(fmt.Sprintf(`{"findings":[{"paths":%s,"severity":"important","message":"m","options":%s,"contests_question_id":%s}]}`,
		quoted, optionsJSON, contests))
}

func TestCheckReview_ContestMustKeepTheQuestionsPathsAndBeAnswerable(t *testing.T) {
	t.Run("item path instead of the field", func(t *testing.T) {
		c, qid := reviewCheckerOn(t, nomPP1)
		_, errs := c.checkReview(findingJSON([]string{string(itemPP1)}, "null", fmt.Sprintf("%q", qid)))
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0], "must include")
	})
	t.Run("no path", func(t *testing.T) {
		c, qid := reviewCheckerOn(t, nomPP1)
		_, errs := c.checkReview(findingJSON(nil, "null", fmt.Sprintf("%q", qid)))
		require.NotEmpty(t, errs)
	})
	t.Run("the field of the question, no option: /answer takes it", func(t *testing.T) {
		c, qid := reviewCheckerOn(t, nomPP1)
		fs, errs := c.checkReview(findingJSON([]string{string(nomPP1)}, "null", fmt.Sprintf("%q", qid)))
		require.Empty(t, errs)
		require.Len(t, fs, 1)
	})
	t.Run("an item question, no option and no plain field: nobody can answer", func(t *testing.T) {
		c, qid := reviewCheckerOn(t, itemPP1)
		_, errs := c.checkReview(findingJSON([]string{string(itemPP1)}, "null", fmt.Sprintf("%q", qid)))
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0], "answer")
	})
	t.Run("an item question with an option is fine", func(t *testing.T) {
		c, qid := reviewCheckerOn(t, itemPP1)
		opt := fmt.Sprintf(`[{"label":"Dupont","fills":[{"path":%q,"value":"Dupont","source":{"field":"last_name","raw_value":"Dupont","table":"persons","object_id":"p1"},"rule_id":null,"justification":null,"confidence":null}]}]`, nomPP1)
		fs, errs := c.checkReview(findingJSON([]string{string(itemPP1)}, opt, fmt.Sprintf("%q", qid)))
		require.Empty(t, errs)
		require.Len(t, fs, 1)
	})
}

// I1: a question the review replaced is not asked again by the job-end pass.
func TestReview_ContestInAGenerationJobIsNotUndoneByTheJobEnd(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withForeignRegister(""))
	f.Format.Waves = [][]core.TaskSpec{{Spec(core.TaskKindReview, tgtPP1)}}
	contested := new(uuid.UUID)
	f.Runner.On(core.TaskKindReview, tgtPP1, Step{
		Before: func() {
			for _, q := range questionsOf(t, f, models.SarQuestionStatusOpen) {
				if len(q.Paths) == 1 && q.Paths[0] == pmRegistrePay {
					*contested = q.Id
				}
			}
		},
		Out: core.TaskOutput{Findings: []core.ReviewFinding{{
			Paths: []core.Path{pmRegistrePay}, Severity: "important", Message: "le pays est lisible",
			ContestsQuestionID: contested,
			Options:            []core.QuestionOption{{Label: "HK", Fills: []core.Fill{{Path: pmRegistrePay, Value: "HK"}}}},
		}}},
	})
	require.NoError(t, o.RunJob(ctx, f.JobId))
	require.NotEqual(t, uuid.Nil, *contested, "the pre-review pass asked the question")

	var open, replaced int
	for _, q := range questionsOf(t, f, "") {
		if len(q.Paths) != 1 || q.Paths[0] != pmRegistrePay {
			continue
		}
		switch q.Status {
		case models.SarQuestionStatusOpen:
			open++
			assert.Equal(t, models.SarQuestionKindReviewFinding, q.Kind)
		case models.SarQuestionStatusReplaced:
			replaced++
		}
	}
	assert.Equal(t, 1, open, "exactly one open item on the path: the finding")
	assert.Equal(t, 1, replaced)
}

// I2: a review re-run never closes a blocking finding, and gives it to the model
// as an open item it must not repeat.
func TestReview_ReRunKeepsOpenBlockingFindings(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	reviewJob(t, f)
	blocking, minor := uuid.New(), uuid.New()
	require.NoError(t, f.Repo.CreateSarQuestions(ctx, nil, []models.SarQuestion{
		{Id: blocking, DeclarationId: f.DeclId, Kind: models.SarQuestionKindReviewFinding, Severity: models.SarQuestionSeverityBlocking,
			Paths: []string{pathNomPP1}, Text: "bloquant", Status: models.SarQuestionStatusOpen},
		{Id: minor, DeclarationId: f.DeclId, Kind: models.SarQuestionKindReviewFinding, Severity: "mineur",
			Paths: []string{"personnes.pp[pp-2]"}, Text: "mineur", Status: models.SarQuestionStatusOpen},
	}))
	require.NoError(t, o.RunJob(ctx, f.JobId))

	status := map[uuid.UUID]string{}
	for _, q := range questionsOf(t, f, "") {
		status[q.Id] = q.Status
	}
	assert.Equal(t, models.SarQuestionStatusOpen, status[blocking], "a blocking finding stays open")
	assert.NotEqual(t, models.SarQuestionStatusOpen, status[minor], "a minor one is replaced by the new review")

	tc := f.Runner.TCs[len(f.Runner.TCs)-1]
	require.Len(t, tc.OpenFindings, 1)
	assert.Equal(t, blocking, tc.OpenFindings[0].Id)
	for _, q := range tc.OpenQuestions {
		assert.Equal(t, models.SarQuestionKindQuestion, q.Kind, "findings are not listed with the questions")
	}
}

func TestReview_OpenQuestionsGivenToTheModelAreCapped(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	reviewJob(t, f)
	var qs []models.SarQuestion
	for i := 0; i < 120; i++ {
		qs = append(qs, models.SarQuestion{Id: uuid.New(), DeclarationId: f.DeclId, Kind: models.SarQuestionKindQuestion,
			Paths: []string{fmt.Sprintf("work.transactions[transactions-%d].montant", i+1)}, Text: "q", Status: models.SarQuestionStatusOpen})
	}
	require.NoError(t, f.Repo.CreateSarQuestions(ctx, nil, qs))
	require.NoError(t, o.RunJob(ctx, f.JobId))

	tc := f.Runner.TCs[len(f.Runner.TCs)-1]
	assert.Len(t, tc.OpenQuestions, 100)
	assert.Equal(t, 20, tc.OpenQuestionsOmitted)
	out := (&agentRunner{}).prompt("P", tc, core.TaskSpec{Kind: core.TaskKindReview}, nil, modeReview)
	assert.Contains(t, out, "20 more open questions are not listed")
}

func TestBackendQuestions_BlockingQuestionNobodyCanAnswerIsNotCreated(t *testing.T) {
	ctx := context.Background()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	f.Format.DocQuestionsFn = func(*core.Document) []core.QuestionDraft {
		return []core.QuestionDraft{
			{Paths: []core.Path{itemPP1}, Text: "bloquant sans issue", Blocking: true},
			{Paths: []core.Path{nomPP1}, Text: "bloquant avec un champ", Blocking: true},
		}
	}
	require.NoError(t, o.RunJob(ctx, f.JobId))
	var texts []string
	for _, q := range questionsOf(t, f, "") {
		texts = append(texts, q.Text)
	}
	assert.Equal(t, []string{"bloquant avec un champ"}, texts)
}
