package sargen

import (
	"bytes"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// A free answer to a question writes like a manual edit and closes the question.

func TestAnswerQuestion_SinglePath(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.general.serviceEmetteur"}, "Quel est le libellé ?", nil)
	version := f.Repo.Declarations[declId].Version

	st, err := uc.AnswerQuestion(f.Ctx, q.Id, version, "Banque Test", "")
	require.NoError(t, err)

	assert.Equal(t, version+1, st.Declaration.Version)
	v := st.Doc.Values["context.general.serviceEmetteur"]
	assert.Equal(t, "Banque Test", v.Value)
	assert.Equal(t, core.OriginManual, v.Origin)
	assert.True(t, v.Reviewed)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status)
	assert.Empty(t, f.JobsOf(declId), "no job is created")
}

func TestAnswerQuestion_MultiPath(t *testing.T) {
	paths := []core.Path{"context.general.serviceEmetteur", "context.general.motifRequisition"}

	t.Run("with a path of the question", func(t *testing.T) {
		uc, f, declId := genReady(t, false)
		q := f.SeedQuestion(t, declId, paths, "Identité ?", nil)
		other := f.SeedQuestion(t, declId, []core.Path{"context.general.motifRequisition"}, "Numéro ?", nil)
		version := f.Repo.Declarations[declId].Version

		st, err := uc.AnswerQuestion(f.Ctx, q.Id, version, "12", "context.general.motifRequisition")
		require.NoError(t, err)
		assert.Equal(t, "12", st.Doc.Values["context.general.motifRequisition"].Value)
		assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[q.Id].Status, "the answered question is closed even if another of its paths is empty")
		assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[other.Id].Status, "a question whose paths are now all filled is closed")
	})

	t.Run("without a path it is refused", func(t *testing.T) {
		uc, f, declId := genReady(t, false)
		q := f.SeedQuestion(t, declId, paths, "Identité ?", nil)
		version := f.Repo.Declarations[declId].Version
		_, err := uc.AnswerQuestion(f.Ctx, q.Id, version, "12", "")
		requireSarCode(t, err, models.SarCodeInvalidRequest)
		assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
		assert.Equal(t, version, f.Repo.Declarations[declId].Version)
	})

	t.Run("a path that is not the question's is refused", func(t *testing.T) {
		uc, f, declId := genReady(t, false)
		q := f.SeedQuestion(t, declId, paths, "Identité ?", nil)
		version := f.Repo.Declarations[declId].Version
		_, err := uc.AnswerQuestion(f.Ctx, q.Id, version, "x", "context.information.activiteLPS")
		requireSarCode(t, err, models.SarCodeInvalidRequest)
		assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
	})
}

func TestAnswerQuestion_NotAPlainField(t *testing.T) {
	for name, path := range map[string]core.Path{
		"list item":      "personnes.pp[pp-1]",
		"group":          "organisme",
		"setting":        "organisme.libelle",
		"computed":       "context.general.refInterne",
		"analyst text":   "analyse",
		"list container": "personnes.pp",
	} {
		t.Run(name, func(t *testing.T) {
			uc, f, declId := genReady(t, false)
			q := f.SeedQuestion(t, declId, []core.Path{path}, "Question ?", nil)
			version := f.Repo.Declarations[declId].Version
			docBefore := append([]byte(nil), f.Repo.Declarations[declId].Document...)

			_, err := uc.AnswerQuestion(f.Ctx, q.Id, version, "x", "")
			requireSarCode(t, err, models.SarCodeInvalidRequest)
			assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
			assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[declId].Document))
		})
	}
}

func TestAnswerQuestion_InvalidValue(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.information.activiteLPS"}, "LPS ?", nil)
	version := f.Repo.Declarations[declId].Version

	_, err := uc.AnswerQuestion(f.Ctx, q.Id, version, "peut-être", "")
	requireSarCode(t, err, models.SarCodeInvalidField)
	var ie *IssuesError
	require.ErrorAs(t, err, &ie)
	require.NotEmpty(t, ie.SarIssues())
	assert.Equal(t, core.Path("context.information.activiteLPS"), ie.SarIssues()[0].Path)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
	assert.Equal(t, version, f.Repo.Declarations[declId].Version)
}

func TestAnswerQuestion_StaleVersion(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.general.serviceEmetteur"}, "Libellé ?", nil)
	version := f.Repo.Declarations[declId].Version

	_, err := uc.AnswerQuestion(f.Ctx, q.Id, version-1, "Banque", "")
	requireSarCode(t, err, models.SarCodeVersionConflict)
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

func TestAnswerQuestion_ClosedQuestion(t *testing.T) {
	uc, f, declId := genReady(t, false)
	q := f.SeedQuestion(t, declId, []core.Path{"context.general.serviceEmetteur"}, "Libellé ?", nil)
	version := f.Repo.Declarations[declId].Version
	_, err := uc.AnswerQuestion(f.Ctx, q.Id, version, "Banque", "")
	require.NoError(t, err)

	_, err = uc.AnswerQuestion(f.Ctx, q.Id, version+1, "Autre", "")
	requireSarCode(t, err, models.SarCodeQuestionClosed)
}

func TestAnswerQuestion_BlockingQuestionNoLongerBlocksExport(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	q := f.SeedQuestion(t, d.Id, []core.Path{"context.general.serviceEmetteur"}, "Libellé ?", nil)
	q.Severity = models.SarQuestionSeverityBlocking
	f.Gen.Questions[q.Id] = q

	_, err := uc.Export(f.Ctx, d.Id, d.Version)
	requireSarCode(t, err, models.SarCodeExportBlocked)

	st, err := uc.AnswerQuestion(f.Ctx, q.Id, d.Version, "Banque Test", "")
	require.NoError(t, err)
	_, err = uc.Export(f.Ctx, d.Id, st.Declaration.Version)
	require.NoError(t, err)
}

func TestAnswerQuestion_ReviewFinding(t *testing.T) {
	uc, f, declId := genReady(t, false)
	finding := f.SeedReviewFinding(t, declId, []core.Path{"context.general.serviceEmetteur"}, "important", "Libellé à vérifier")
	version := f.Repo.Declarations[declId].Version

	st, err := uc.AnswerQuestion(f.Ctx, finding.Id, version, "Banque Test", "")
	require.NoError(t, err)
	assert.Equal(t, "Banque Test", st.Doc.Values["context.general.serviceEmetteur"].Value)
	assert.Equal(t, models.SarQuestionStatusResolved, f.Gen.Questions[finding.Id].Status)
}

func TestAnswerQuestion_OtherOrgForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	foreign := models.SarDeclaration{
		Id: uuid.New(), OrganizationId: uuid.New(), CaseId: f.CaseId,
		FormatKey: tracfinFormatKey, ReportType: tracfinReportType, Version: 1,
		Document: []byte(`{"values":{},"items":{},"item_sources":{},"next_key":{}}`),
	}
	require.NoError(t, f.Repo.CreateSarDeclaration(f.Ctx, nil, foreign))
	docBefore := append([]byte(nil), foreign.Document...)
	q := f.SeedQuestion(t, foreign.Id, []core.Path{"context.general.serviceEmetteur"}, "Libellé ?", nil)

	_, err := uc.AnswerQuestion(f.Ctx, q.Id, 1, "Banque", "")
	require.ErrorIs(t, err, models.ForbiddenError)
	assert.True(t, bytes.Equal(docBefore, f.Repo.Declarations[foreign.Id].Document))
	assert.Equal(t, models.SarQuestionStatusOpen, f.Gen.Questions[q.Id].Status)
}

// requireSarCode checks that err is a refusal carrying the given stable code.
func requireSarCode(t *testing.T, err error, code models.SarErrorCode) {
	t.Helper()
	require.Error(t, err)
	var ie *IssuesError
	if errors.As(err, &ie) {
		assert.Equal(t, code, ie.SarCode())
		return
	}
	var se *models.SarError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, code, se.Code)
}
