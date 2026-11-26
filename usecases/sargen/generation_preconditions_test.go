package sargen

import (
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Every failed precondition is reported at once, as issues with a code and a
// path the front maps: analysis, selection and each missing setting.
func TestStartGeneration_AllFailedPreconditionsAreIssues(t *testing.T) {
	uc, f := newTestUsecase(t)
	declId := createFresh(t, uc, f).Declaration.Id // no analysis, no selection, no settings

	_, err := uc.StartGeneration(f.Ctx, declId)

	require.ErrorIs(t, err, models.BadParameterError)
	var ie *IssuesError
	require.True(t, errors.As(err, &ie), "the refusal carries issues: %v", err)
	assert.Equal(t, models.SarCodeGenerationPrecondition, ie.SarCode())
	codes := map[string][]core.Path{}
	for _, i := range ie.SarIssues() {
		assert.Equal(t, core.SeverityBlocking, i.Severity)
		assert.NotEmpty(t, i.Message)
		codes[i.Code] = append(codes[i.Code], i.Path)
	}
	assert.Equal(t, []core.Path{PreconditionAnalysisPath}, codes[CodePreconditionAnalysisEmpty])
	assert.Equal(t, []core.Path{PreconditionSelectionPath}, codes[CodePreconditionSelectionEmpty])
	assert.Contains(t, codes[CodePreconditionSettingMissing], core.Path("organisme.libelle"))
	assert.Empty(t, f.JobsOf(declId), "no job must be created")
}

func TestStartGeneration_InvalidSettingIsAnIssueOnItsPath(t *testing.T) {
	uc, f, declId := genReady(t, false)
	invalid := genCompleteSettings()
	invalid["organisme.adresse.pays"] = "X"
	genSetSettings(f, invalid)

	_, err := uc.StartGeneration(f.Ctx, declId)

	var ie *IssuesError
	require.True(t, errors.As(err, &ie), "%v", err)
	require.Len(t, ie.SarIssues(), 1)
	assert.Equal(t, core.Path("organisme.adresse.pays"), ie.SarIssues()[0].Path)
	assert.Equal(t, CodePreconditionSettingInvalid, ie.SarIssues()[0].Code)
}

func TestStartGeneration_PreconditionMessageHasNoTechnicalPath(t *testing.T) {
	uc, f := newTestUsecase(t)
	declId := createFresh(t, uc, f).Declaration.Id

	_, err := uc.StartGeneration(f.Ctx, declId)

	var ie *IssuesError
	require.True(t, errors.As(err, &ie), "%v", err)
	assert.Regexp(t, `^la génération est impossible : l'analyse est vide ; la sélection est vide ; \d+ réglages manquants ou invalides$`, ie.Error())
	assert.NotContains(t, ie.Error(), "organisme")
	assert.Contains(t, ie.SarIssues()[len(ie.SarIssues())-1].Path, "organisme")
}

// errorAndIssuePaths is the message of err plus the paths of its issues (the
// message itself carries no technical path).
func errorAndIssuePaths(err error) string {
	out := err.Error()
	var ie *IssuesError
	if errors.As(err, &ie) {
		for _, i := range ie.SarIssues() {
			out += " " + string(i.Path)
		}
	}
	return out
}
