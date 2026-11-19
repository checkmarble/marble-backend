package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Questions whose option adds a list item (provisional key) and the option that
// declines the addition.

const (
	addFirstNamePath = "personnes.pp[pp-1].etatCivil.prenom[+f]"
	addQuestionText  = "Faut-il ajouter un prénom ?"
)

func addQuestionDraft(path string) core.QuestionDraft {
	return core.QuestionDraft{
		Paths: []core.Path{core.Path(path)}, Text: addQuestionText,
		Options: []core.QuestionOption{
			{Label: "Ajouter", Fills: []core.Fill{{Path: core.Path(path), Value: "Marie"}}},
			{Label: "Ne pas ajouter"},
		},
	}
}

func runAddQuestion(t *testing.T, d core.QuestionDraft) (*fakes, []core.QuestionOption, []string) {
	t.Helper()
	o, f := newTestOrchestrator(t)
	seedDocument(t, f, withTwoPersons)
	f.Format.Waves = [][]core.TaskSpec{{Spec("a", tgtPP1)}}
	f.Runner.On("a", tgtPP1, Step{Out: core.TaskOutput{Questions: []core.QuestionDraft{d}}})
	require.NoError(t, o.RunJob(context.Background(), f.JobId))
	qs := questionsOf(t, f, "")
	if len(qs) == 0 {
		return f, nil, nil
	}
	require.Len(t, qs, 1)
	var opts []core.QuestionOption
	if len(qs[0].Options) > 0 {
		require.NoError(t, json.Unmarshal(qs[0].Options, &opts))
	}
	return f, opts, qs[0].Paths
}

func TestRunJob_AddQuestionKeepsProvisionalPathAndBothOptions(t *testing.T) {
	_, opts, paths := runAddQuestion(t, addQuestionDraft(addFirstNamePath))

	require.Equal(t, []string{addFirstNamePath}, paths, "the provisional key is stored as is")
	require.Len(t, opts, 2)
	require.Len(t, opts[0].Fills, 1)
	assert.Equal(t, core.Path(addFirstNamePath), opts[0].Fills[0].Path)
	assert.Equal(t, core.OriginLLM, opts[0].Fills[0].Origin)
	assert.Empty(t, opts[1].Fills, "the declining option is kept, with no fill")
	assert.Equal(t, "Ne pas ajouter", opts[1].Label)
}

func TestRunJob_AddQuestionDoesNotWriteTheDocument(t *testing.T) {
	f, _, _ := runAddQuestion(t, addQuestionDraft(addFirstNamePath))
	assert.Empty(t, f.Repo.Doc(t).Items["personnes.pp[pp-1].etatCivil.prenom"], "nothing created before the analyst chooses")
}

func TestRunJob_EmptyOptionDroppedWithoutAddingSibling(t *testing.T) {
	d := core.QuestionDraft{
		Paths: []core.Path{pathImplPP1}, Text: "Implication ?",
		Options: []core.QuestionOption{
			{Label: "Oui", Fills: []core.Fill{{Path: pathImplPP1, Value: "v"}}},
			{Label: "Je ne sais pas"},
		},
	}
	_, opts, _ := runAddQuestion(t, d)
	require.Len(t, opts, 1, "an option that writes nothing stays dropped when no sibling adds an item")
	assert.Equal(t, "Oui", opts[0].Label)
}

func TestRunJob_ProvisionalPathOutsideAllowedPrefixStillRejected(t *testing.T) {
	// pp-2 is outside the task's part (pp-1).
	_, opts, paths := runAddQuestion(t, addQuestionDraft("personnes.pp[pp-2].etatCivil.prenom[+f]"))
	assert.Empty(t, paths)
	assert.Empty(t, opts, "the question is path-less: its options cannot write and are dropped")
}

func TestRunJob_ProvisionalPathOnNonRepeatableStillRejected(t *testing.T) {
	_, _, paths := runAddQuestion(t, addQuestionDraft("personnes.pp[pp-1].etatCivil.nomNaissance[+f]"))
	assert.Empty(t, paths)
}

func TestRunJob_OptionFillOnAnotherProvisionalKeyRejected(t *testing.T) {
	d := addQuestionDraft(addFirstNamePath)
	d.Options[0].Fills[0].Path = "personnes.pp[pp-1].etatCivil.prenom[+g]"
	_, opts, _ := runAddQuestion(t, d)
	assert.Empty(t, opts, "a fill off the question's own paths is dropped, so no option creates an item")
}

func TestAgent_AddOptionAndDecliningOptionAreKept(t *testing.T) {
	final := `{"fills":[],"questions":[{"paths":["` + addFirstNamePath + `"],"text":"Prénom ?",
	 "options":[{"label":"Ajouter","fills":[{"path":"` + addFirstNamePath + `","value":"Marie","source":null}]},{"label":"Ne pas ajouter"}]}]}`
	llm := newScriptedLLM(FinalStep(final))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	require.Len(t, out.Questions, 1)
	q := out.Questions[0]
	assert.Equal(t, []core.Path{addFirstNamePath}, q.Paths)
	require.Len(t, q.Options, 2)
	assert.Equal(t, "Ajouter", q.Options[0].Label)
	require.Len(t, q.Options[0].Fills, 1)
	assert.Equal(t, core.OriginAnalyst, q.Options[0].Fills[0].Origin)
	assert.Equal(t, "Ne pas ajouter", q.Options[1].Label)
	assert.Empty(t, q.Options[1].Fills)
}

func TestAgent_EmptyOptionDroppedWhenNoSiblingAddsAnItem(t *testing.T) {
	final := optionQuestion(agNomPath, `{"path":"`+agNomPath+`","value":"Dupont","source":null}`)
	llm := newScriptedLLM(FinalStep(final))
	out, err := agRun(t, llm, fakeAgentPrompts{}, AgentLimits{}, agentToolContext(t), agentSpec("fill", agentPersonPath))
	require.NoError(t, err)

	require.Len(t, out.Questions, 1)
	require.Len(t, out.Questions[0].Options, 1, "« Non » writes nothing and nothing is added: dropped")
	assert.Equal(t, "Oui", out.Questions[0].Options[0].Label)
}
