package sargen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const closingField = core.Path("context.general.serviceEmetteur")

// An open finding of any severity is closed once all its fields are filled, by
// an answer to another question, by choosing an option or by a manual edit.
func TestFindingClosesWhenItsFieldsAreFilled(t *testing.T) {
	for _, severity := range []string{"important", "mineur", models.SarQuestionSeverityBlocking} {
		t.Run(severity+" by an answer", func(t *testing.T) {
			uc, f, declId := genReady(t, false)
			q := f.SeedQuestion(t, declId, []core.Path{closingField}, "Libellé ?", nil)
			finding := f.SeedReviewFinding(t, declId, []core.Path{closingField}, severity, "à vérifier")
			_, err := uc.AnswerQuestion(f.Ctx, q.Id, f.Repo.Declarations[declId].Version, "Banque Test", "")
			require.NoError(t, err)
			assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[finding.Id].Status)
		})
		t.Run(severity+" by a manual edit", func(t *testing.T) {
			uc, f, declId := genReady(t, false)
			finding := f.SeedReviewFinding(t, declId, []core.Path{closingField}, severity, "à vérifier")
			_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
				[]core.Fill{{Path: closingField, Value: "Banque Test"}})
			require.NoError(t, err)
			assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[finding.Id].Status)
		})
	}
}

// A finding stays open while one of its fields is empty, and a finding with no
// path is never closed automatically.
func TestFindingStaysOpenWhileAFieldIsEmptyOrWithoutPath(t *testing.T) {
	uc, f, declId := genReady(t, false)
	two := f.SeedReviewFinding(t, declId, []core.Path{closingField, "context.general.motifRequisition"}, "important", "deux champs")
	none := f.SeedReviewFinding(t, declId, nil, "mineur", "sans chemin")
	_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		[]core.Fill{{Path: closingField, Value: "Banque Test"}})
	require.NoError(t, err)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[two.Id].Status)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[none.Id].Status)
}
