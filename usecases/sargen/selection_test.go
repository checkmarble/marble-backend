package sargen

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/utils"
)

// Shared helpers for the Task 9 tests (selection, candidates, navigation, analysis).

const (
	selFormatKey  = "tracfin:FR:1.4.0"
	selReportType = "DS_INITIALE"
)

// selSetup creates a TRACFIN declaration on the test case (version 1).
func selSetup(t *testing.T) (*Usecase, *fakes, uuid.UUID) {
	t.Helper()
	uc, f := newTestUsecase(t)
	st, err := uc.CreateDeclaration(f.Ctx, f.CaseId, selFormatKey, selReportType)
	require.NoError(t, err)
	require.Equal(t, 1, st.Declaration.Version)
	return uc, f, st.Declaration.Id
}

func selRef(table, objectId string) core.ObjectRef {
	return core.ObjectRef{Table: table, ObjectID: objectId}
}

// selOtherOrgCtx carries credentials of an organization that owns nothing here.
func selOtherOrgCtx() context.Context {
	return context.WithValue(context.Background(), utils.ContextKeyCredentials,
		models.Credentials{OrganizationId: uuid.New()})
}

// selAddCaseEntity stores an ingested object and adds it to the case entities.
func selAddCaseEntity(f *fakes, table, objectId string, data map[string]any) {
	f.Ingested.Put(table, objectId, data, time.Now())
	f.CaseEntities.Refs = append(f.CaseEntities.Refs, models.CaseEntityRef{TableName: table, ObjectId: objectId})
}

// selDataModel: transactions -[account]-> accounts (to single), and accounts
// -> transactions by account_id (navigation option, to many).
func selDataModel(f *fakes) {
	f.DataModel.Model = models.DataModel{Tables: map[string]models.Table{
		"transactions": {
			Name: "transactions",
			LinksToSingle: map[string]models.LinkToSingle{
				"account": {
					Name:            "account",
					ChildTableName:  "transactions",
					ChildFieldName:  "account_id",
					ParentTableName: "accounts",
					ParentFieldName: "object_id",
				},
			},
		},
		"accounts": {
			Name: "accounts",
			NavigationOptions: []models.NavigationOption{{
				SourceTableName:   "accounts",
				SourceFieldName:   "object_id",
				TargetTableName:   "transactions",
				FilterFieldName:   "account_id",
				OrderingFieldName: "object_id",
				Status:            models.IndexStatusValid,
			}},
		},
	}}
}

// ---- UpdateSelection ----

func TestUpdateSelection_UnreachableObjectForbidden(t *testing.T) {
	uc, f, declId := selSetup(t)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1"}, time.Now())
	// a9 exists in the ingested data but is neither a candidate nor reached through a link.
	f.Ingested.Put("accounts", "a9", map[string]any{"object_id": "a9"}, time.Now())

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("accounts", "a9")}}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, models.ForbiddenError)

	assert.Empty(t, f.Selection.Rows, "nothing must be selected")
	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, 1, st.Declaration.Version, "version must not change")
}

func TestUpdateSelection_TriggerObjectSnapshotComesFromDecision(t *testing.T) {
	uc, f, declId := selSetup(t)
	// The trigger object is NOT in the ingested data: the snapshot must come from the decision.
	decisionId := f.Decisions.Add(f.OrgId, "transactions",
		map[string]any{"object_id": "t1", "amount": 42}, time.Now())

	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)

	require.Len(t, f.Selection.Rows, 1)
	row := f.Selection.Rows[0]
	assert.Equal(t, "transactions", row.TableName)
	assert.Equal(t, "t1", row.ObjectId)
	assert.EqualValues(t, 42, row.Snapshot["amount"])
	assert.Equal(t, "t1", row.Snapshot["object_id"])
	assert.Equal(t, models.SarOriginKindDecision, row.Origin.Kind)
	require.NotNil(t, row.Origin.DecisionId)
	assert.Equal(t, decisionId, *row.Origin.DecisionId)
	assert.Equal(t, 2, st.Declaration.Version)
}

func TestUpdateSelection_SameTriggerObjectInTwoDecisionsUsesLatest(t *testing.T) {
	uc, f, declId := selSetup(t)
	now := time.Now()
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "amount": 1}, now.Add(-time.Hour))
	newerId := f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "amount": 2}, now)

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)

	require.Len(t, f.Selection.Rows, 1)
	row := f.Selection.Rows[0]
	assert.EqualValues(t, 2, row.Snapshot["amount"], "the most recent decision must provide the snapshot")
	require.NotNil(t, row.Origin.DecisionId)
	assert.Equal(t, newerId, *row.Origin.DecisionId)
}

func TestSnapshotSurvivesSourceDeletion(t *testing.T) {
	uc, f, declId := selSetup(t)
	selAddCaseEntity(f, "accounts", "a1", map[string]any{"object_id": "a1", "name": "Acme"})

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("accounts", "a1")}}, nil)
	require.NoError(t, err)

	// The client data disappears after the selection.
	f.Ingested.SetDeleted(true)

	rows, err := uc.selectionRepo.ListSarSelection(f.Ctx, nil, declId)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Acme", rows[0].Snapshot["name"])

	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, declId, st.Declaration.Id)

	// Candidates and navigation from the selected object still work.
	selDataModel(f)
	cands, err := uc.ListCandidates(f.Ctx, declId)
	require.NoError(t, err)
	for _, c := range cands {
		if c.Ref == selRef("accounts", "a1") {
			assert.True(t, c.Selected)
		}
	}
	_, _, err = uc.Navigate(f.Ctx, declId, selRef("accounts", "a1"), "transactions.account_id", nil, 10)
	require.Error(t, err, "reading the deleted source fails, but the selected object is accepted as origin")
	assert.NotErrorIs(t, err, models.ForbiddenError)
}

func TestUpdateSelection_IncrementsVersionAndStaleVersionConflicts(t *testing.T) {
	uc, f, declId := selSetup(t)
	selAddCaseEntity(f, "accounts", "a1", map[string]any{"object_id": "a1"})
	selAddCaseEntity(f, "accounts", "a2", map[string]any{"object_id": "a2"})

	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("accounts", "a1")}}, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, st.Declaration.Version)

	// Version 1 is stale now.
	_, err = uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("accounts", "a2")}}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, models.ConflictError)
	assert.Len(t, f.Selection.Rows, 1, "a stale update must not write anything")

	st, err = uc.UpdateSelection(f.Ctx, declId, 2, []SelectionAdd{{Ref: selRef("accounts", "a2")}}, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, st.Declaration.Version)
	assert.Len(t, f.Selection.Rows, 2)
}

func TestUpdateSelection_ReachedThroughLink(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	// t1 is a trigger object of a decision and points to account a1 through the "account" link.
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a1"}, time.Now())
	f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1", "name": "Acme"}, time.Now())

	reached := &models.SarReachedFrom{TableName: "transactions", ObjectId: "t1", Link: "account"}
	st, err := uc.UpdateSelection(f.Ctx, declId, 1,
		[]SelectionAdd{{Ref: selRef("accounts", "a1"), ReachedFrom: reached}}, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, st.Declaration.Version)

	require.Len(t, f.Selection.Rows, 1)
	row := f.Selection.Rows[0]
	assert.Equal(t, "accounts", row.TableName)
	assert.Equal(t, "a1", row.ObjectId)
	require.NotNil(t, row.ReachedFrom)
	assert.Equal(t, *reached, *row.ReachedFrom)
	assert.Equal(t, models.SarOriginKindIngested, row.Origin.Kind)
	assert.Equal(t, "Acme", row.Snapshot["name"])
}

func TestUpdateSelection_RemoveObject(t *testing.T) {
	uc, f, declId := selSetup(t)
	selAddCaseEntity(f, "accounts", "a1", map[string]any{"object_id": "a1"})

	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("accounts", "a1")}}, nil)
	require.NoError(t, err)
	require.Len(t, f.Selection.Rows, 1)

	st, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, nil, []core.ObjectRef{selRef("accounts", "a1")})
	require.NoError(t, err)
	assert.Empty(t, f.Selection.Rows)
	assert.Equal(t, 3, st.Declaration.Version)
}

// ---- UpdateAnalysis ----

func TestUpdateAnalysis_WritesAnalyseField(t *testing.T) {
	uc, f, declId := selSetup(t)

	st, err := uc.UpdateAnalysis(f.Ctx, declId, 1, "Analyse du soupçon.", "Informations complémentaires.")
	require.NoError(t, err)

	assert.Equal(t, 2, st.Declaration.Version)
	assert.Equal(t, "Analyse du soupçon.", st.Declaration.Analysis)
	assert.Equal(t, "Informations complémentaires.", st.Declaration.ExtraInfo)

	stored := f.Repo.Declarations[declId]
	assert.Equal(t, "Analyse du soupçon.", stored.Analysis)
	assert.Equal(t, "Informations complémentaires.", stored.ExtraInfo)
	assert.Equal(t, 2, stored.Version)

	require.NotNil(t, st.Doc)
	v, ok := st.Doc.Values[core.Path("analyse")]
	require.True(t, ok, "the analyse field must be written in the document")
	assert.Equal(t, "Analyse du soupçon.", v.Value)
	assert.Equal(t, core.OriginAnalyst, v.Origin)
}

// ---- security (negative) ----

func TestUpdateSelection_NegativeLinkCases(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		add     func(f *fakes) SelectionAdd
		wantErr error
	}{
		{
			name: "reached from an object that is neither candidate nor selected",
			add: func(f *fakes) SelectionAdd {
				f.Ingested.Put("transactions", "t9", map[string]any{"object_id": "t9", "account_id": "a1"}, now)
				return SelectionAdd{Ref: selRef("accounts", "a1"),
					ReachedFrom: &models.SarReachedFrom{TableName: "transactions", ObjectId: "t9", Link: "account"}}
			},
			wantErr: models.ForbiddenError,
		},
		{
			name: "unknown link name",
			add: func(f *fakes) SelectionAdd {
				return SelectionAdd{Ref: selRef("accounts", "a1"),
					ReachedFrom: &models.SarReachedFrom{TableName: "transactions", ObjectId: "t1", Link: "nope"}}
			},
			wantErr: models.ForbiddenError,
		},
		{
			name: "link not targeting the table of the object",
			add: func(f *fakes) SelectionAdd {
				f.Ingested.Put("customers", "c1", map[string]any{"object_id": "c1"}, now)
				return SelectionAdd{Ref: selRef("customers", "c1"),
					ReachedFrom: &models.SarReachedFrom{TableName: "transactions", ObjectId: "t1", Link: "account"}}
			},
			wantErr: models.ForbiddenError,
		},
		{
			name: "object exists but is not the one the link leads to",
			add: func(f *fakes) SelectionAdd {
				return SelectionAdd{Ref: selRef("accounts", "a2"),
					ReachedFrom: &models.SarReachedFrom{TableName: "transactions", ObjectId: "t1", Link: "account"}}
			},
			wantErr: models.ForbiddenError,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, f, declId := selSetup(t)
			selDataModel(f)
			f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a1"}, now)
			f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1"}, now)
			f.Ingested.Put("accounts", "a2", map[string]any{"object_id": "a2"}, now)
			add := tc.add(f)

			_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{add}, nil)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.Empty(t, f.Selection.Rows)
			assert.Equal(t, 1, f.Repo.Declarations[declId].Version)
		})
	}
}

func TestUpdateSelection_OtherOrgForbidden(t *testing.T) {
	uc, f, declId := selSetup(t)
	selAddCaseEntity(f, "accounts", "a1", map[string]any{"object_id": "a1"})

	_, err := uc.UpdateSelection(selOtherOrgCtx(), declId, 1, []SelectionAdd{{Ref: selRef("accounts", "a1")}}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, models.ForbiddenError)
	assert.Empty(t, f.Selection.Rows)
}

func TestUpdateSelection_DedupesAddAndRejectsAddRemoveOfSameRef(t *testing.T) {
	uc, f, declId := selSetup(t)
	selAddCaseEntity(f, "accounts", "a1", map[string]any{"object_id": "a1"})
	ref := selRef("accounts", "a1")

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: ref}, {Ref: ref}}, nil)
	require.NoError(t, err)
	assert.Len(t, f.Selection.Rows, 1)

	_, err = uc.UpdateSelection(f.Ctx, declId, 2, []SelectionAdd{{Ref: ref}}, []core.ObjectRef{ref})
	require.Error(t, err)
	assert.ErrorIs(t, err, models.BadParameterError)
	assert.Equal(t, 2, f.Repo.Declarations[declId].Version)
}

func TestUpdateAnalysis_EmptyClearsField(t *testing.T) {
	uc, f, declId := selSetup(t)
	_, err := uc.UpdateAnalysis(f.Ctx, declId, 1, "Texte", "")
	require.NoError(t, err)

	st, err := uc.UpdateAnalysis(f.Ctx, declId, 2, "", "")
	require.NoError(t, err)
	v := st.Doc.Values[core.Path("analyse")]
	assert.Equal(t, "", v.Value)
	assert.Equal(t, "", st.Declaration.Analysis)
}

func TestAnalystLeaf(t *testing.T) {
	uc, f, declId := selSetup(t)
	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	path, err := analystLeaf(st.Format.Spec())
	require.NoError(t, err)
	assert.Equal(t, core.Path("analyse"), path)

	none, err := analystLeaf(&core.Spec{Root: &core.Node{}})
	require.NoError(t, err)
	assert.Equal(t, core.Path(""), none)

	two, err := analystLeaf(&core.Spec{Root: &core.Node{Children: []*core.Node{
		{Template: "a", Kind: core.NodeField, Nature: core.NatureAnalyst},
		{Template: "b", Kind: core.NodeField, Nature: core.NatureAnalyst},
	}}})
	require.Error(t, err)
	assert.Equal(t, core.Path(""), two)
}
