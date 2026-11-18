package sargen

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Security tests of the declaration usecase, run on the organization- and
// case-aware fakes. Each test covers one check; removing that check makes the
// test fail.

func TestSecurity_CreateDeclarationOnSiblingCaseAllowed(t *testing.T) {
	// Positive control: another case of the same organization is accessible.
	uc, f := newTestUsecase(t)
	st, err := uc.CreateDeclaration(f.Ctx, f.SiblingCaseId, selFormatKey, selReportType)
	require.NoError(t, err)
	assert.Equal(t, f.SiblingCaseId, st.Declaration.CaseId)
}

func TestSecurity_CreateDeclarationOnCaseOfAnotherOrgForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	_, err := uc.CreateDeclaration(f.Ctx, f.OtherCaseId, selFormatKey, selReportType)
	require.ErrorIs(t, err, models.ForbiddenError)
	assert.Empty(t, f.Repo.Declarations, "no declaration may be created on another organization's case")
}

func TestSecurity_ListDeclarationsHidesOtherOrgDeclarations(t *testing.T) {
	uc, f, declId := selSetup(t)
	foreign := models.SarDeclaration{
		Id:             uuid.New(),
		OrganizationId: f.OtherOrgId,
		CaseId:         f.CaseId,
		FormatKey:      selFormatKey,
		ReportType:     selReportType,
		Version:        1,
		Document:       []byte(`{}`),
	}
	require.NoError(t, f.Repo.CreateSarDeclaration(f.Ctx, nil, foreign))

	list, err := uc.ListDeclarations(f.Ctx, f.CaseId)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, declId, list[0].Id)
}

// Candidates are scoped to the organization and the case of the declaration:
// decisions of another case or of another organization are not listed, nor
// are the entities of another case.
func TestSecurity_CandidatesStayInOrgAndCase(t *testing.T) {
	uc, f, declId := selSetup(t)
	now := time.Now()
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1"}, now)
	f.Decisions.AddInCase(f.OrgId, f.SiblingCaseId.String(), "transactions",
		map[string]any{"object_id": "t2"}, now)
	f.Decisions.AddInCase(f.OtherOrgId, f.CaseId.String(), "transactions",
		map[string]any{"object_id": "t3"}, now)
	selAddCaseEntity(f, "transactions", "t4", map[string]any{"object_id": "t4"})

	cands, err := uc.ListCandidates(f.Ctx, declId)
	require.NoError(t, err)
	refs := make([]core.ObjectRef, 0, len(cands))
	for _, c := range cands {
		refs = append(refs, c.Ref)
	}
	assert.ElementsMatch(t, []core.ObjectRef{selRef("transactions", "t1"), selRef("transactions", "t4")}, refs)

	sibling, err := uc.CreateDeclaration(f.Ctx, f.SiblingCaseId, selFormatKey, selReportType)
	require.NoError(t, err)
	siblingCands, err := uc.ListCandidates(f.Ctx, sibling.Declaration.Id)
	require.NoError(t, err)
	require.Len(t, siblingCands, 1)
	assert.Equal(t, selRef("transactions", "t2"), siblingCands[0].Ref)
}

// Objects of another organization with the same identifier are never read.
func TestSecurity_ObjectsOfAnotherOrgAreNotReachable(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	now := time.Now()
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a1"}, now)
	f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1", "name": "Acme"}, now)
	f.Ingested.PutInOrg(f.OtherOrgId, "accounts", "a1", map[string]any{"object_id": "a1", "name": "Autre organisation"}, now)

	got, _, err := uc.Navigate(f.Ctx, declId, selRef("transactions", "t1"), "account", nil, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Acme", got[0].Summary["name"])
}

// The locked row is checked against the caller's organization inside the
// transaction: a row that changed under the lock must not be written.
func TestSecurity_UpdateSelectionLockedRowOfAnotherOrgForbidden(t *testing.T) {
	uc, f, declId := selSetup(t)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1"}, time.Now())
	f.Repo.OnLock = func(_ *fakeRepo, d *models.SarDeclaration) { d.OrganizationId = f.OtherOrgId }

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.ErrorIs(t, err, models.ForbiddenError)
	assert.Empty(t, f.Selection.Rows, "no object may be selected when the locked row is refused")
	assert.Equal(t, 1, f.Repo.Declarations[declId].Version)
}

// The version is checked again under the lock: a write committed between the
// unlocked read and the lock must not leave selected rows behind.
func TestSecurity_UpdateSelectionStaleLockedVersionWritesNothing(t *testing.T) {
	uc, f, declId := selSetup(t)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1"}, time.Now())
	f.Repo.OnLock = func(r *fakeRepo, d *models.SarDeclaration) {
		stored := r.Declarations[d.Id]
		stored.Version++
		r.Declarations[d.Id] = stored
		d.Version = stored.Version
	}

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.ErrorIs(t, err, models.ErrSarVersionConflict)
	assert.Empty(t, f.Selection.Rows, "a stale write must not leave selected rows behind")
	assert.Equal(t, 2, f.Repo.Declarations[declId].Version)
}

func TestSecurity_UpdateFieldsLockedRowOfAnotherOrgForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)
	id, base := st.Declaration.Id, st.Declaration.Version
	before := f.Repo.Declarations[id]
	f.Repo.OnLock = func(_ *fakeRepo, d *models.SarDeclaration) { d.OrganizationId = f.OtherOrgId }

	_, err := uc.UpdateFields(f.Ctx, id, base, []core.Fill{{Path: "declarant.coordonnees.prenom", Value: "Paul"}})
	require.ErrorIs(t, err, models.ForbiddenError)
	assert.Equal(t, base, f.Repo.Declarations[id].Version, "no version bump for a refused row")
	assert.Equal(t, before.Document, f.Repo.Declarations[id].Document)
}

func TestSecurity_ExportLockedRowOfAnotherOrgForbidden(t *testing.T) {
	uc, f := newTestUsecase(t)
	d := seedValidDeclaration(t, uc, f)
	f.Repo.OnLock = func(_ *fakeRepo, cp *models.SarDeclaration) { cp.OrganizationId = f.OtherOrgId }

	_, err := uc.Export(f.Ctx, d.Id, d.Version)
	require.ErrorIs(t, err, models.ForbiddenError)
	assert.Empty(t, f.Blobs.Keys(), "no archive may be written for a row of another organization")
	assert.Empty(t, f.Repo.Exports)
}

// The readers are called with the organization of the declaration only, and
// they serve only that organization's data.
func TestSecurity_ReadersUseDeclarationOrganization(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	base := time.Now().Add(-time.Hour)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a1"}, base)
	f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1", "name": "Acme"}, base)

	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	_, _, err = uc.Navigate(f.Ctx, declId, selRef("transactions", "t1"), "account", nil, 10)
	require.NoError(t, err)
	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, []SelectionAdd{{
		Ref:         selRef("accounts", "a1"),
		ReachedFrom: &models.SarReachedFrom{TableName: "transactions", ObjectId: "t1", Link: "account"},
	}}, nil)
	require.NoError(t, err)

	require.NotEmpty(t, f.Ingested.OrgIds)
	for _, o := range f.Ingested.OrgIds {
		assert.Equal(t, f.OrgId, o, "every ingested read must use the declaration organization")
	}
	require.NotEmpty(t, f.DataModel.Orgs)
	for _, o := range f.DataModel.Orgs {
		assert.Equal(t, f.OrgId, o, "every data model read must use the declaration organization")
	}
}
