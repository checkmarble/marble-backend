package sargen

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	removedItem  = core.Path("personnes.pp[pp-1]")
	removedField = core.Path("personnes.pp[pp-1].etatCivil.nomNaissance")
)

func seedFinding(t *testing.T, f *fakes, declId uuid.UUID, severity string, paths []core.Path, opts []core.QuestionOption) models.SarQuestion {
	t.Helper()
	q := f.SeedQuestion(t, declId, paths, "constat", opts)
	q.Kind = models.SarQuestionKindReviewFinding
	q.Severity = severity
	f.Gen.Questions[q.Id] = q
	return q
}

func openOn(f *fakes, path core.Path) []models.SarQuestion {
	var out []models.SarQuestion
	for _, q := range f.Gen.Questions {
		if q.Status != models.SarQuestionStatusOpen {
			continue
		}
		for _, p := range q.Paths {
			if p == string(path) {
				out = append(out, q)
				break
			}
		}
	}
	return out
}

// R1: a blocking finding about an item that is removed has nothing left to answer:
// it is closed and the export is no longer blocked by it.
func TestRemoveItem_ClosesTheBlockingFindingOfThatItem(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	q := seedFinding(t, f, d.Id, models.SarQuestionSeverityBlocking, []core.Path{removedField}, nil)
	requireExportBlockedBy(t, uc, f, d, "constat")

	_, err := uc.RemoveItem(f.Ctx, d.Id, f.Repo.Declarations[d.Id].Version, removedItem)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusDismissed, f.Gen.Questions[q.Id].Status)
	_, err = uc.Export(f.Ctx, d.Id, f.Repo.Declarations[d.Id].Version)
	require.NoError(t, err)
}

// Only some paths inside the item: they are dropped and the item stays open with
// the rest (its options pruned).
func TestRemoveItem_DropsThePathsOfTheRemovedItemAndKeepsTheRest(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	opts := []core.QuestionOption{
		{Label: "Les deux", Fills: []core.Fill{{Path: removedField, Value: "Dupont"}, {Path: "context.information.activiteLPS", Value: "true"}}},
		{Label: "Seulement la personne", Fills: []core.Fill{{Path: removedField, Value: "Dupont"}}},
	}
	q := seedFinding(t, f, d.Id, models.SarQuestionSeverityBlocking,
		[]core.Path{removedField, "context.information.activiteLPS"}, opts)

	_, err := uc.RemoveItem(f.Ctx, d.Id, f.Repo.Declarations[d.Id].Version, removedItem)
	require.NoError(t, err)
	assert.NotEqual(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status, "the old row is closed")

	kept := openOn(f, "context.information.activiteLPS")
	require.Len(t, kept, 1)
	assert.Equal(t, []string{"context.information.activiteLPS"}, kept[0].Paths)
	assert.Equal(t, models.SarQuestionKindReviewFinding, kept[0].Kind)
	assert.Equal(t, models.SarQuestionSeverityBlocking, kept[0].Severity)
	qq, err := questionFromModel(kept[0])
	require.NoError(t, err)
	require.Len(t, qq.Options, 1, "the option that only wrote the removed person is gone")
	require.Len(t, qq.Options[0].Fills, 1)
	assert.Equal(t, core.Path("context.information.activiteLPS"), qq.Options[0].Fills[0].Path)
	assert.Empty(t, openOn(f, removedField))
}

// What is left cannot be answered (a setting, no option): the item is closed too.
func TestRemoveItem_ClosesWhatIsLeftWhenItCannotBeAnswered(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	q := seedFinding(t, f, d.Id, models.SarQuestionSeverityBlocking, []core.Path{removedField, "organisme.libelle"}, nil)

	_, err := uc.RemoveItem(f.Ctx, d.Id, f.Repo.Declarations[d.Id].Version, removedItem)
	require.NoError(t, err)
	assert.NotEqual(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
	assert.Empty(t, openOn(f, "organisme.libelle"))
}

// The removal through a question's option does the same.
func TestChooseRemoval_ClosesTheFindingsOfTheRemovedItem(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	ask := f.SeedQuestion(t, d.Id, []core.Path{removedItem}, "Déclarer ?",
		[]core.QuestionOption{{Label: "Déclarer"}, {Label: "Ne pas déclarer", RemoveItem: removedItem}})
	finding := seedFinding(t, f, d.Id, models.SarQuestionSeverityBlocking, []core.Path{removedField}, nil)

	_, err := uc.ChooseQuestionOption(f.Ctx, ask.Id, f.Repo.Declarations[d.Id].Version, 1)
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusDismissed, f.Gen.Questions[finding.Id].Status)
}

// AnswerQuestion and the shared rule accept the same paths.
func TestAnswerQuestion_PathRuleIsTheSharedRule(t *testing.T) {
	for _, path := range []core.Path{
		"context.information.activiteLPS", "work.sensMontantTotal", removedField,
		"organisme.libelle", "context.general.refInterne", "operations.montantTotal", "analyse",
		removedItem, "personnes.pp", "organisme", "nope.nothing",
	} {
		t.Run(string(path), func(t *testing.T) {
			uc, f, declId := genReady(t, false)
			format, err := uc.formatOf(f.Repo.Declarations[declId])
			require.NoError(t, err)
			q := f.SeedQuestion(t, declId, []core.Path{path}, "Question ?", nil)
			_, err = uc.AnswerQuestion(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, "x", "")
			refusedByTheRule := false
			var se *models.SarError
			if errors.As(err, &se) {
				refusedByTheRule = se.Code == models.SarCodeInvalidRequest
			}
			var ie *IssuesError
			if errors.As(err, &ie) {
				refusedByTheRule = ie.SarCode() == models.SarCodeInvalidRequest
			}
			assert.Equal(t, core.FieldAnswerable(format, path), !refusedByTheRule, "same rule")
		})
	}
}
