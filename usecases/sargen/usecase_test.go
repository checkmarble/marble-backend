package sargen

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats"
	"github.com/checkmarble/marble-backend/utils"
)

// TRACFIN fixture constants and the testFormat wrapper below are test fixtures
// only: package sargen itself holds no TRACFIN knowledge.
const (
	tracfinFamily     = "tracfin"
	tracfinFormatKey  = "tracfin:FR:1.4.0"
	tracfinReportType = "DS_INITIALE"
	// minimalDocumentPath is a complete, blocking-free TRACFIN pivot document.
	minimalDocumentPath = "formats/tracfin/testdata/minimal.json"
)

// testFormat wraps the real TRACFIN 1.4.0 format: status can be overridden
// (deprecated / retired) and extra issues are appended to Validate's output.
type testFormat struct {
	core.Format
	status core.VersionStatus
	extra  []core.Issue
}

func (f testFormat) Status() core.VersionStatus {
	if f.status != "" {
		return f.status
	}
	return f.Format.Status()
}

func (f testFormat) Validate(doc *core.Document) []core.Issue {
	return append(f.Format.Validate(doc), f.extra...)
}

// useFormat makes uc resolve every format key to the wrapped TRACFIN format.
func useFormat(t *testing.T, uc *Usecase, wrap testFormat) {
	t.Helper()
	base, ok := formats.Get(core.FormatKey{Family: tracfinFamily, Country: "FR", Version: "1.4.0"})
	require.True(t, ok, "TRACFIN 1.4.0 must be a known format")
	wrap.Format = base
	uc.formatGetter = func(core.FormatKey) (core.Format, bool) { return wrap, true }
}

// createFresh creates an empty TRACFIN declaration on the test case.
func createFresh(t *testing.T, uc *Usecase, f *fakes) DeclarationState {
	t.Helper()
	st, err := uc.CreateDeclaration(f.Ctx, f.CaseId, tracfinFormatKey, tracfinReportType)
	require.NoError(t, err)
	return st
}

// seedValidDeclaration creates a declaration and replaces its document by the
// minimal complete TRACFIN document (no blocking issue).
func seedValidDeclaration(t *testing.T, uc *Usecase, f *fakes) models.SarDeclaration {
	t.Helper()
	st := createFresh(t, uc, f)
	raw, err := os.ReadFile(minimalDocumentPath)
	require.NoError(t, err)
	d := f.Repo.Declarations[st.Declaration.Id]
	d.Document = raw
	f.Repo.Declarations[d.Id] = d
	return d
}

func decodeDoc(t *testing.T, raw []byte) *core.Document {
	t.Helper()
	var doc core.Document
	require.NoError(t, json.Unmarshal(raw, &doc))
	return &doc
}

func isForbiddenOrNotFound(err error) bool {
	return errors.Is(err, models.ForbiddenError) || errors.Is(err, models.NotFoundError)
}

func TestCreateDeclaration_ForbiddenCase(t *testing.T) {
	uc, f := newTestUsecase(t)
	f.Security.Forbid()

	_, err := uc.CreateDeclaration(f.Ctx, f.CaseId, tracfinFormatKey, tracfinReportType)
	require.ErrorIs(t, err, models.ForbiddenError)
	require.Empty(t, f.Repo.Declarations)
}

func TestCreateDeclaration_UnknownFormat(t *testing.T) {
	uc, f := newTestUsecase(t)

	_, err := uc.CreateDeclaration(f.Ctx, f.CaseId, "tracfin:FR:9.9.9", tracfinReportType)
	require.ErrorIs(t, err, models.BadParameterError)
	require.Empty(t, f.Repo.Declarations)
}

func TestCreateDeclaration_NonActiveVersionRefused(t *testing.T) {
	uc, f := newTestUsecase(t)
	existing := seedValidDeclaration(t, uc, f)

	// The version becomes deprecated after the declaration was opened.
	useFormat(t, uc, testFormat{status: core.VersionDeprecated})

	_, err := uc.CreateDeclaration(f.Ctx, f.CaseId, tracfinFormatKey, tracfinReportType)
	require.ErrorIs(t, err, models.BadParameterError)
	require.Len(t, f.Repo.Declarations, 1, "no new declaration may be created on a deprecated version")

	st, err := uc.GetDeclaration(f.Ctx, existing.Id)
	require.NoError(t, err, "an existing declaration on a deprecated version stays readable")
	require.Equal(t, existing.Id, st.Declaration.Id)

	exp, err := uc.Export(f.Ctx, existing.Id, st.Declaration.Version)
	require.NoError(t, err, "an existing declaration on a deprecated version stays exportable")
	require.NotEqual(t, uuid.Nil, exp.Id)
}

func TestCreateDeclaration_AppliesOrgSettings(t *testing.T) {
	uc, f := newTestUsecase(t)
	f.Repo.Settings[settingsKey(f.OrgId, tracfinFamily)] = models.SarOrgFormatSettings{
		OrganizationId: f.OrgId,
		FormatFamily:   tracfinFamily,
		Values: map[string]string{
			"organisme.libelle":         "Banque Exemple",
			"declarant.coordonnees.nom": "Dupont",
		},
	}

	st := createFresh(t, uc, f)

	v, ok := st.Doc.Values["organisme.libelle"]
	require.True(t, ok, "org setting must be applied at creation")
	require.Equal(t, "Banque Exemple", v.Value)
	require.Equal(t, core.OriginOrgSetting, v.Origin)

	n, ok := st.Doc.Values["declarant.coordonnees.nom"]
	require.True(t, ok)
	require.Equal(t, "Dupont", n.Value)
	require.Equal(t, core.OriginOrgSetting, n.Origin)
}

func TestCreateDeclaration_AppliesFormatDefaults(t *testing.T) {
	uc, f := newTestUsecase(t)

	st := createFresh(t, uc, f)

	ref, ok := st.Doc.Values["context.general.refInterne"]
	require.True(t, ok, "default refInterne must be set")
	require.Equal(t, f.CaseId.String(), ref.Value, "the {case_id} marker must be replaced by the case id")
	require.Equal(t, core.OriginComputed, ref.Origin)

	for _, p := range []core.Path{"context.general.estPrecisionSignalement", "context.general.estRequisitionJudiciaire"} {
		v, ok := st.Doc.Values[p]
		require.True(t, ok, "default %s must be set", p)
		require.Equal(t, "false", v.Value)
		require.Equal(t, core.OriginComputed, v.Origin)
	}
}

func TestUpdateFields_AllowedAfterExport(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	exp1, err := uc.Export(f.Ctx, d.Id, d.Version)
	require.NoError(t, err)
	firstDoc := append([]byte(nil), exp1.Document...)

	st, err := uc.UpdateFields(f.Ctx, d.Id, d.Version, []core.Fill{
		{Path: "declarant.coordonnees.prenom", Value: "Paul"},
	})
	require.NoError(t, err, "a declaration stays editable after an export")
	require.Greater(t, st.Declaration.Version, d.Version)

	exp2, err := uc.Export(f.Ctx, d.Id, st.Declaration.Version)
	require.NoError(t, err)
	require.NotEqual(t, exp1.Id, exp2.Id, "each export is a new archive")
	require.NotEqual(t, exp1.DeclarationVersion, exp2.DeclarationVersion)

	stored, ok := f.Repo.Exports[exp1.Id]
	require.True(t, ok)
	require.Equal(t, firstDoc, stored.Document, "the first export's document copy must not change")
	require.NotEqual(t, firstDoc, exp2.Document)

	got, err := uc.GetDeclaration(f.Ctx, d.Id)
	require.NoError(t, err)
	require.Equal(t, 2, got.ExportsCount)
	require.NotNil(t, got.LastExportAt)
}

func TestUpdateFields_StaleVersionConflict(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)
	id, base := st.Declaration.Id, st.Declaration.Version

	after, err := uc.UpdateFields(f.Ctx, id, base, []core.Fill{
		{Path: "declarant.coordonnees.nom", Value: "Dupont"},
	})
	require.NoError(t, err)
	require.Greater(t, after.Declaration.Version, base)

	_, err = uc.UpdateFields(f.Ctx, id, base, []core.Fill{
		{Path: "declarant.coordonnees.prenom", Value: "Paul"},
	})
	require.ErrorIs(t, err, models.ErrSarVersionConflict)
	require.Equal(t, after.Declaration.Version, f.Repo.Declarations[id].Version, "stale write must not bump the version")
}

func TestUpdateFields_RejectedBatchSavesNothing(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)

	_, err := uc.UpdateFields(f.Ctx, st.Declaration.Id, st.Declaration.Version, []core.Fill{
		{Path: "declarant.coordonnees.nom", Value: "Dupont"},
		{Path: "operations.dateDebut", Value: "pas une date"},
	})
	require.ErrorIs(t, err, models.BadParameterError)

	got, err := uc.GetDeclaration(f.Ctx, st.Declaration.Id)
	require.NoError(t, err)
	require.Equal(t, st.Declaration.Version, got.Declaration.Version, "version must be unchanged")
	_, ok := got.Doc.Values["declarant.coordonnees.nom"]
	require.False(t, ok, "the valid fill of a rejected batch must not be saved")
}

func TestUpdateFields_ComputedLineFieldRejected(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	_, err := uc.UpdateFields(f.Ctx, d.Id, d.Version, []core.Fill{
		{Path: "operations.operation[op-1].montantTotal", Value: "100.00"},
	})
	require.ErrorIs(t, err, models.BadParameterError)
	require.ErrorContains(t, err, "computed_line")
}

func TestUpdateFields_NonLineComputedFieldAllowed(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	st, err := uc.UpdateFields(f.Ctx, d.Id, d.Version, []core.Fill{
		{Path: "operations.montantTotal", Value: "13000.00"},
	})
	require.NoError(t, err, "operations.montantTotal is computed but editable (not inside a line)")
	require.Equal(t, "13000.00", st.Doc.Values["operations.montantTotal"].Value)
}

func TestExport_RefusedWithBlockingIssues(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f) // mandatory TRACFIN fields are missing

	_, err := uc.Export(f.Ctx, st.Declaration.Id, st.Declaration.Version)
	require.ErrorIs(t, err, models.BadParameterError)
	require.Empty(t, f.Blobs.Keys(), "nothing may be written to the bucket")
	require.Empty(t, f.Repo.Exports)
}

func TestExport_AllowedWithWarningsAndRecordsOutstanding(t *testing.T) {
	uc, f := newTestUsecase(t)
	warning := core.Issue{
		Path:     "operations.dateFin",
		Code:     "test_warning",
		Message:  "avertissement de test",
		Severity: core.SeverityWarning,
	}
	useFormat(t, uc, testFormat{extra: []core.Issue{warning}})

	d := seedValidDeclaration(t, uc, f)
	// One low-confidence value that has not been reviewed.
	doc := decodeDoc(t, d.Document)
	v := doc.Values["declarant.coordonnees.prenom"]
	v.Confidence = core.ConfidenceLow
	v.Reviewed = false
	doc.Values["declarant.coordonnees.prenom"] = v
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	d.Document = raw
	f.Repo.Declarations[d.Id] = d
	question := f.SeedQuestion(t, d.Id, []core.Path{"operations.dateFin"}, "Date de fin à établir ?", nil)
	finding := f.SeedReviewFinding(t, d.Id, []core.Path{"declarant.coordonnees.nom"}, "important", "Nom à relire")

	exp, err := uc.Export(f.Ctx, d.Id, d.Version)
	require.NoError(t, err, "warnings and unreviewed low-confidence values do not block the export")
	require.Contains(t, string(exp.Outstanding), question.Text, "an open question must be recorded")
	require.Contains(t, string(exp.Outstanding), finding.Text, "an open review finding must be recorded")
	require.Contains(t, string(exp.Outstanding), "test_warning", "remaining warning must be recorded")
	require.Contains(t, string(exp.Outstanding), "declarant.coordonnees.prenom",
		"unreviewed low-confidence value must be recorded")
}

func TestExport_DeprecatedVersionWarns(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	useFormat(t, uc, testFormat{status: core.VersionDeprecated})

	exp, err := uc.Export(f.Ctx, d.Id, d.Version)
	require.NoError(t, err, "a deprecated version still exports")
	require.Contains(t, string(exp.Outstanding), "warning", "the deprecation must be recorded as a warning")
	require.Len(t, f.Blobs.Keys(), 1)
}

func TestExport_RetiredVersionRefused(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	useFormat(t, uc, testFormat{status: core.VersionRetired})

	_, err := uc.Export(f.Ctx, d.Id, d.Version)
	require.ErrorIs(t, err, models.BadParameterError)
	require.Empty(t, f.Blobs.Keys())
	require.Empty(t, f.Repo.Exports)
}

func TestExport_WritesArchive(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	exp, err := uc.Export(f.Ctx, d.Id, d.Version)
	require.NoError(t, err)

	key := fmt.Sprintf("%s/%s/sar-declarations/%s/%s.xml",
		f.OrgId.String(), f.CaseId.String(), d.Id.String(), exp.Id.String())
	require.Equal(t, []string{key}, f.Blobs.Keys())

	content, ok := f.Blobs.Content(key)
	require.True(t, ok)
	require.True(t, bytes.Contains(content, []byte("<ds")), "the archive must be the XML document")

	sum := sha256.Sum256(content)
	require.Equal(t, hex.EncodeToString(sum[:]), exp.Sha256)

	stored, ok := f.Repo.Exports[exp.Id]
	require.True(t, ok, "export row must be recorded")
	require.Equal(t, d.Id, stored.DeclarationId)
	require.Equal(t, d.Version, stored.DeclarationVersion)
	require.Equal(t, f.Repo.Declarations[d.Id].Document, stored.Document, "the document copy must be recorded")
}

func TestDownloadExport_OtherDeclarationExportForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	a := seedValidDeclaration(t, uc, f)
	expA, err := uc.Export(f.Ctx, a.Id, a.Version)
	require.NoError(t, err)

	blob, err := uc.DownloadExport(f.Ctx, a.Id, expA.Id)
	require.NoError(t, err)
	require.NotNil(t, blob.ReadCloser)

	other := createFresh(t, uc, f)
	_, err = uc.DownloadExport(f.Ctx, other.Declaration.Id, expA.Id)
	require.Error(t, err)
	require.True(t, isForbiddenOrNotFound(err), "got %v", err)
}

func TestGetDeclaration_OtherOrgForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	foreign := models.SarDeclaration{
		Id:             uuid.New(),
		OrganizationId: uuid.New(), // another organization
		CaseId:         f.CaseId,
		FormatKey:      tracfinFormatKey,
		ReportType:     tracfinReportType,
		Version:        1,
		Document:       []byte(`{"values":{},"items":{},"item_sources":{},"next_key":{}}`),
	}
	require.NoError(t, f.Repo.CreateSarDeclaration(f.Ctx, nil, foreign))

	_, err := uc.GetDeclaration(f.Ctx, foreign.Id)
	require.Error(t, err)
	require.True(t, isForbiddenOrNotFound(err), "got %v", err)
}

func TestPutOrgSettings_RejectsNonSettingPath(t *testing.T) {
	uc, f := newTestUsecase(t)

	// context.general.refInterne is a computed field, not an organization setting.
	_, _, err := uc.PutOrgSettings(f.Ctx, tracfinFamily, map[string]string{
		"context.general.refInterne": "REF-1",
	})
	require.ErrorIs(t, err, models.BadParameterError)
	_, stored := f.Repo.Settings[settingsKey(f.OrgId, tracfinFamily)]
	require.False(t, stored, "a rejected settings write must not be stored")
}

func TestMarkReviewed_SetsReviewed(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)
	path := core.Path("context.general.refInterne")
	require.False(t, st.Doc.Values[path].Reviewed)

	after, err := uc.MarkReviewed(f.Ctx, st.Declaration.Id, st.Declaration.Version, []core.Path{path})
	require.NoError(t, err)
	require.True(t, after.Doc.Values[path].Reviewed)
	require.Greater(t, after.Declaration.Version, st.Declaration.Version)
}

func TestAddItem_RemoveItem(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)
	id := st.Declaration.Id

	item, added, err := uc.AddItem(f.Ctx, id, st.Declaration.Version, "personnes.pp")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(item), "personnes.pp["), "item %s must sit in the container", item)
	require.Len(t, added.Doc.Items["personnes.pp"], 1)

	removed, err := uc.RemoveItem(f.Ctx, id, added.Declaration.Version, item)
	require.NoError(t, err)
	require.Empty(t, removed.Doc.Items["personnes.pp"])
}

func TestPutOrgSettings_ScopedByCredentialsOrg(t *testing.T) {
	uc, f := newTestUsecase(t)
	ctxB := context.WithValue(context.Background(), utils.ContextKeyCredentials,
		models.Credentials{OrganizationId: uuid.New()})

	_, _, err := uc.PutOrgSettings(f.Ctx, tracfinFamily, map[string]string{"organisme.libelle": "Banque A"})
	require.NoError(t, err)

	gotB, _, err := uc.GetOrgSettings(ctxB, tracfinFamily)
	require.NoError(t, err)
	require.Empty(t, gotB.Values, "org B must not see org A settings")

	_, _, err = uc.PutOrgSettings(ctxB, tracfinFamily, map[string]string{"organisme.libelle": "Banque B"})
	require.NoError(t, err)

	gotA, _, err := uc.GetOrgSettings(f.Ctx, tracfinFamily)
	require.NoError(t, err)
	require.Equal(t, "Banque A", gotA.Values["organisme.libelle"], "org B must not overwrite org A settings")
}

func TestPutOrgSettings_InvalidValueReturnsIssuesAndSavesNothing(t *testing.T) {
	uc, f := newTestUsecase(t)

	_, issues, err := uc.PutOrgSettings(f.Ctx, tracfinFamily, map[string]string{
		"declarant.coordonnees.nom": strings.Repeat("x", 5000),
	})
	require.ErrorIs(t, err, models.BadParameterError)
	require.NotEmpty(t, issues)
	_, stored := f.Repo.Settings[settingsKey(f.OrgId, tracfinFamily)]
	require.False(t, stored)
}

func TestExport_StaleVersionConflict(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	_, err := uc.Export(f.Ctx, d.Id, d.Version+7)
	require.ErrorIs(t, err, models.ErrSarVersionConflict)
	require.Empty(t, f.Blobs.Keys())
	require.Empty(t, f.Repo.Exports)
}

func TestDownloadExport_UnknownExportNotFound(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	_, err := uc.DownloadExport(f.Ctx, d.Id, uuid.New())
	require.ErrorIs(t, err, models.NotFoundError)
}

func TestRemoveItem_UnknownItemNotFound(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)

	_, err := uc.RemoveItem(f.Ctx, st.Declaration.Id, st.Declaration.Version, "personnes.pp[nope-1]")
	require.ErrorIs(t, err, models.NotFoundError)
	require.Equal(t, st.Declaration.Version, f.Repo.Declarations[st.Declaration.Id].Version)
}

func TestAddItem_MissingParentRefused(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)

	_, _, err := uc.AddItem(f.Ctx, st.Declaration.Id, st.Declaration.Version, "personnes.pp[nope-1].adresse")
	require.Error(t, err)
	require.Equal(t, st.Declaration.Version, f.Repo.Declarations[st.Declaration.Id].Version)
}

func TestUpdateFields_ManualEditKeepsOnlyPathAndValue(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	taskId := uuid.New()

	st, err := uc.UpdateFields(f.Ctx, d.Id, d.Version, []core.Fill{{
		Path: "declarant.coordonnees.nom", Value: "Durand", Origin: core.OriginLLM,
		Reviewed: true, Confidence: core.ConfidenceHigh, Justification: "j", RuleIDs: []string{"r1"},
		Sources:     []core.SourceRef{{Object: core.ObjectRef{Table: "t", ObjectID: "1"}, Field: "x", RawValue: "y"}},
		Computation: &core.Computation{Name: "sum"}, TaskID: &taskId,
	}})
	require.NoError(t, err)

	v := st.Doc.Values["declarant.coordonnees.nom"]
	require.Equal(t, "Durand", v.Value)
	require.Equal(t, core.OriginManual, v.Origin)
	require.True(t, v.Reviewed, "a manual value is stored reviewed")
	require.Empty(t, v.Sources)
	require.Empty(t, v.RuleIDs)
	require.Empty(t, v.Justification)
	require.Equal(t, core.ConfidenceNone, v.Confidence)
	require.Nil(t, v.Computation)
	require.Nil(t, v.TaskID)
}

func TestUpdateFields_AnalystPathRejected(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	_, err := uc.UpdateFields(f.Ctx, d.Id, d.Version, []core.Fill{{Path: "analyse", Value: "texte"}})
	require.ErrorIs(t, err, models.BadParameterError)
	require.ErrorContains(t, err, "mise à jour de l'analyse")
	require.Equal(t, d.Version, f.Repo.Declarations[d.Id].Version)
}

func TestAddItem_ChecksEveryKeyedAncestor(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	const missing = "personnes.pp[nope-1].referencesFinancieres.referenceFinanciereIban"

	_, _, err := uc.AddItem(f.Ctx, d.Id, d.Version, core.Path(missing))
	require.ErrorIs(t, err, models.BadParameterError)
	require.Equal(t, d.Version, f.Repo.Declarations[d.Id].Version)

	item, _, err := uc.AddItem(f.Ctx, d.Id, d.Version,
		"personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban")
	require.NoError(t, err, "the existing parent item is accepted")
	require.NotEmpty(t, item)
}

func TestAddRemoveItem_ComputedContainerRefused(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)

	_, _, err := uc.AddItem(f.Ctx, d.Id, d.Version, "operations.operation")
	require.ErrorIs(t, err, models.BadParameterError)
	require.ErrorContains(t, err, "calculée")

	_, err = uc.RemoveItem(f.Ctx, d.Id, d.Version, "operations.operation[op-1]")
	require.ErrorIs(t, err, models.BadParameterError)
	require.ErrorContains(t, err, "calculée")
	require.Equal(t, d.Version, f.Repo.Declarations[d.Id].Version)
}

func TestNewerVersion_Semantic(t *testing.T) {
	require.True(t, newerVersion("1.10.0", "1.4.0"))
	require.False(t, newerVersion("1.4.0", "1.10.0"))
	require.False(t, newerVersion("1.4.0", "1.4.0"))
	require.True(t, newerVersion("2.0.0", "1.99.9"))
}

func TestListExports_ReturnsExportsAndChecksOrg(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	exp, err := uc.Export(f.Ctx, d.Id, d.Version)
	require.NoError(t, err)

	list, err := uc.ListExports(f.Ctx, d.Id)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, exp.Id, list[0].Id)

	_, err = uc.ListExports(selOtherOrgCtx(), d.Id)
	require.ErrorIs(t, err, models.ForbiddenError)
}

func TestExport_BlobDeletedWhenExportRowCannotBeCreated(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	f.Repo.FailExportCreate = errors.New("db down")

	_, err := uc.Export(f.Ctx, d.Id, d.Version)
	require.Error(t, err)
	require.Empty(t, f.Blobs.Keys(), "the orphan blob must be deleted")
	require.Empty(t, f.Repo.Exports)
}
