package sargen

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// chainTable builds a table with an object_id, one foreign key and one link to
// the parent table.
func chainTable(name string, sem models.SemanticType, fkField, parent string, fields map[string]models.Field) models.Table {
	t := models.Table{
		Name: name, SemanticType: sem,
		Fields:        map[string]models.Field{"object_id": {Name: "object_id", DataType: models.String}},
		LinksToSingle: map[string]models.LinkToSingle{},
	}
	for k, v := range fields {
		t.Fields[k] = v
	}
	if parent != "" {
		t.Fields[fkField] = models.Field{Name: fkField, DataType: models.String}
		t.LinksToSingle[parent] = models.LinkToSingle{
			Name: parent, ChildTableName: name, ChildFieldName: fkField,
			ParentTableName: parent, ParentFieldName: "object_id",
		}
	}
	return t
}

// parentsModel: transactions -> accounts -> persons -> companies -> groups
// (depth 4 from a transaction), plus transactions -> channels (not useful).
func parentsModel(f *fakes) {
	f.DataModel.Model = models.DataModel{Tables: map[string]models.Table{
		"transactions": chainTable("transactions", models.SemanticTypeTransaction, "account_id", "accounts", nil),
		"accounts":     chainTable("accounts", models.SemanticTypeAccount, "person_id", "persons", nil),
		"persons":      chainTable("persons", models.SemanticTypePerson, "company_id", "companies", nil),
		"companies":    chainTable("companies", models.SemanticTypeCompany, "group_id", "groups", nil),
		"groups":       chainTable("groups", models.SemanticTypeCompany, "", "", nil),
	}}
	f.Ingested.model = f.DataModel
}

func putChain(f *fakes) {
	now := time.Now()
	f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1", "person_id": "p1"}, now)
	f.Ingested.Put("persons", "p1", map[string]any{"object_id": "p1", "company_id": "c1"}, now)
	f.Ingested.Put("companies", "c1", map[string]any{"object_id": "c1", "group_id": "g1"}, now)
	f.Ingested.Put("groups", "g1", map[string]any{"object_id": "g1"}, now)
}

func addTrigger(f *fakes, id, account string) {
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": id, "account_id": account}, time.Now())
}

func selectedTables(f *fakes) map[string]bool {
	out := map[string]bool{}
	for _, r := range f.Selection.Rows {
		out[r.TableName+"/"+r.ObjectId] = r.AutoAdded
	}
	return out
}

func TestAutoParents_DepthThreeReachedDepthFourNot(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	putChain(f)
	addTrigger(f, "t1", "a1")

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	got := selectedTables(f)
	assert.Equal(t, map[string]bool{
		"transactions/t1": false, "accounts/a1": true, "persons/p1": true, "companies/c1": true,
	}, got, "groups is the 4th hop and must not be added")
	for _, r := range f.Selection.Rows {
		if r.AutoAdded {
			require.NotNil(t, r.ReachedFrom)
			assert.Equal(t, models.SarOriginKindIngested, r.Origin.Kind)
		}
	}
	for _, r := range f.Selection.Rows {
		if r.TableName == "persons" {
			assert.Equal(t, "accounts", r.ReachedFrom.TableName)
			assert.Equal(t, "a1", r.ReachedFrom.ObjectId)
			assert.Equal(t, "persons", r.ReachedFrom.Link)
		}
	}
}

func TestAutoParents_NonUsefulTableSkippedButTraversed(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	// accounts becomes a non-useful table (no semantics): traversed, not added.
	acc := f.DataModel.Model.Tables["accounts"]
	acc.SemanticType = models.SemanticTypeUnset
	f.DataModel.Model.Tables["accounts"] = acc
	putChain(f)
	addTrigger(f, "t1", "a1")

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"transactions/t1": false, "persons/p1": true, "companies/c1": true,
	}, selectedTables(f))
}

func TestAutoParents_IdentifyingFieldMakesTableUseful(t *testing.T) {
	assert.False(t, isUsefulParentTable(models.Table{}))
	assert.False(t, isUsefulParentTable(models.Table{SemanticType: models.SemanticTypeEvent,
		Fields: map[string]models.Field{"x": {SemanticType: models.FieldSemanticTypeCurrency}}}))
	assert.True(t, isUsefulParentTable(models.Table{SemanticType: models.SemanticTypeOther,
		Fields: map[string]models.Field{"x": {SemanticType: models.FieldSemanticTypeIban}}}))
	assert.False(t, isUsefulParentTable(models.Table{
		Fields: map[string]models.Field{"x": {SemanticType: models.FieldSemanticTypeIban, Archived: true}}}))
	assert.True(t, isUsefulParentTable(models.Table{SemanticType: models.SemanticTypePartner}))
}

func TestAutoParents_ToManyLinksNeverFollowed(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	acc := f.DataModel.Model.Tables["accounts"]
	acc.SemanticType = models.SemanticTypeAccount
	f.DataModel.Model.Tables["accounts"] = acc
	f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1"}, time.Now())
	f.Ingested.Put("transactions", "t2", map[string]any{"object_id": "t2", "account_id": "a1"}, time.Now())
	f.Decisions.Add(f.OrgId, "accounts", map[string]any{"object_id": "a1"}, time.Now())

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("accounts", "a1")}}, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"accounts/a1": false}, selectedTables(f))
}

func TestAutoParents_CapOfFifty(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	now := time.Now()
	var adds []SelectionAdd
	for i := 0; i < 60; i++ {
		acc, id := fmt.Sprintf("a%02d", i), fmt.Sprintf("t%02d", i)
		f.Ingested.Put("accounts", acc, map[string]any{"object_id": acc}, now)
		addTrigger(f, id, acc)
		adds = append(adds, SelectionAdd{Ref: selRef("transactions", id)})
	}
	_, err := uc.UpdateSelection(f.Ctx, declId, 1, adds, nil)
	require.NoError(t, err)
	auto := 0
	for _, r := range f.Selection.Rows {
		if r.AutoAdded {
			auto++
		}
	}
	assert.Equal(t, maxAutoAddedPerDeclaration, auto)
	assert.Len(t, f.Selection.Rows, 60+50)
	// deterministic: the 50 first accounts by id
	assert.Contains(t, selectedTables(f), "accounts/a49")
	assert.NotContains(t, selectedTables(f), "accounts/a50")
}

func TestAutoParents_ExclusionSurvivesLaterAddition(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	putChain(f)
	addTrigger(f, "t1", "a1")
	addTrigger(f, "t2", "a1")

	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	// The analyst unchecks the auto-added account.
	st, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, nil, []core.ObjectRef{selRef("accounts", "a1")})
	require.NoError(t, err)
	require.Len(t, f.Selection.Exclusions, 1)
	// Persons and companies only led through the account: removed with it.
	assert.Equal(t, map[string]bool{"transactions/t1": false}, selectedTables(f))

	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"transactions/t1": false, "transactions/t2": false}, selectedTables(f),
		"the excluded account must not come back")
}

func TestAutoParents_ExplicitAdditionClearsMarkerAndExclusion(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	putChain(f)
	addTrigger(f, "t1", "a1")

	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	// Adding the auto-added person explicitly: an analyst choice now.
	st, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, []SelectionAdd{{Ref: selRef("persons", "p1")}}, nil)
	require.NoError(t, err)
	assert.False(t, selectedTables(f)["persons/p1"])
	assert.Equal(t, 3, st.Declaration.Version)

	// It survives the removal of its last child.
	st, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, nil, []core.ObjectRef{selRef("transactions", "t1")})
	require.NoError(t, err)
	got := selectedTables(f)
	assert.Contains(t, got, "persons/p1")
	assert.NotContains(t, got, "accounts/a1", "an auto-added parent without child leaves with it")

	// Exclusion then explicit addition clears the exclusion.
	f.Selection.Exclusions = append(f.Selection.Exclusions, models.CaseEntityRef{TableName: "companies", ObjectId: "c1"})
	f.Ingested.Put("companies", "c1", map[string]any{"object_id": "c1"}, time.Now())
	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version,
		[]SelectionAdd{{Ref: selRef("companies", "c1"), ReachedFrom: &models.SarReachedFrom{TableName: "persons", ObjectId: "p1", Link: "companies"}}}, nil)
	require.NoError(t, err)
	assert.Empty(t, f.Selection.Exclusions)
	assert.False(t, selectedTables(f)["companies/c1"])
}

func TestAutoParents_ParentLeftWhileAnotherChildLeadsToIt(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	putChain(f)
	addTrigger(f, "t1", "a1")
	addTrigger(f, "t2", "a1")

	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{
		{Ref: selRef("transactions", "t1")}, {Ref: selRef("transactions", "t2")}}, nil)
	require.NoError(t, err)
	st, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, nil, []core.ObjectRef{selRef("transactions", "t1")})
	require.NoError(t, err)
	assert.Contains(t, selectedTables(f), "accounts/a1")
	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, nil, []core.ObjectRef{selRef("transactions", "t2")})
	require.NoError(t, err)
	assert.Empty(t, f.Selection.Rows)
	assert.Empty(t, f.Selection.Exclusions, "an orphan removal is not an analyst exclusion")
}

func TestAutoParents_CrossOrgAndMissingParentsNeverRead(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	other := f.OtherOrgId
	f.Ingested.PutInOrg(other, "accounts", "a1", map[string]any{"object_id": "a1"}, time.Now())
	addTrigger(f, "t1", "a1") // the account only exists in another organization

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"transactions/t1": false}, selectedTables(f))
	for _, o := range f.Ingested.OrgIds {
		assert.Equal(t, f.OrgId, o, "only the declaration's organization is ever read")
	}
}

func TestAutoParents_SnapshotsFrozenAndUTC(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	loc := time.FixedZone("x", 3600)
	f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1", "opened": time.Date(2026, 1, 1, 10, 0, 0, 0, loc)}, time.Now())
	addTrigger(f, "t1", "a1")

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	f.Ingested.Remove("accounts", "a1")
	for _, r := range f.Selection.Rows {
		if r.TableName == "accounts" {
			assert.Equal(t, "2026-01-01T09:00:00Z", r.Snapshot["opened"])
		}
	}
	require.Contains(t, selectedTables(f), "accounts/a1")
}

func TestAutoParents_CandidateExposesMarkerAndPath(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	putChain(f)
	addTrigger(f, "t1", "a1")
	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)

	objs, _, err := uc.Navigate(f.Ctx, declId, selRef("transactions", "t1"), "accounts", nil, 10)
	require.NoError(t, err)
	require.Len(t, objs, 1)
	assert.True(t, objs[0].Selected)
	assert.True(t, objs[0].AutoAdded)
	require.NotNil(t, objs[0].ReachedFrom)
	assert.Equal(t, "t1", objs[0].ReachedFrom.ObjectId)

	cands, err := uc.ListCandidates(f.Ctx, declId)
	require.NoError(t, err)
	require.Len(t, cands, 1)
	assert.False(t, cands[0].AutoAdded, "the analyst's own choice")
	assert.Nil(t, cands[0].ReachedFrom)
}

func TestAutoParents_ParentBehindANonUsefulTableSurvivesAnUnrelatedAddition(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	acc := f.DataModel.Model.Tables["accounts"]
	acc.SemanticType = models.SemanticTypeUnset
	f.DataModel.Model.Tables["accounts"] = acc
	putChain(f)
	f.Ingested.Put("accounts", "a2", map[string]any{"object_id": "a2"}, time.Now())
	addTrigger(f, "t1", "a1")
	addTrigger(f, "t2", "a2")

	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, []SelectionAdd{{Ref: selRef("transactions", "t2")}}, nil)
	require.NoError(t, err)
	got := selectedTables(f)
	assert.Contains(t, got, "persons/p1", "reached through a table that was not added: still alive")
	assert.Contains(t, got, "companies/c1")
	for _, r := range f.Selection.Rows {
		if r.TableName == "persons" {
			assert.Equal(t, "t1", r.ReachedFrom.ObjectId, "the nearest selected object it was reached from")
		}
	}
}

func TestAutoParents_CycleOfParentsLeavesWithItsLastAnalystChild(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	// companies point back to persons: persons <-> companies.
	co := f.DataModel.Model.Tables["companies"]
	co.Fields["ceo_id"] = models.Field{Name: "ceo_id", DataType: models.String}
	co.LinksToSingle["persons"] = models.LinkToSingle{Name: "persons", ChildTableName: "companies",
		ChildFieldName: "ceo_id", ParentTableName: "persons", ParentFieldName: "object_id"}
	f.DataModel.Model.Tables["companies"] = co
	putChain(f)
	f.Ingested.Put("companies", "c1", map[string]any{"object_id": "c1", "group_id": "g1", "ceo_id": "p1"}, time.Now())
	addTrigger(f, "t1", "a1")

	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	require.Contains(t, selectedTables(f), "companies/c1")
	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, nil, []core.ObjectRef{selRef("transactions", "t1")})
	require.NoError(t, err)
	assert.Empty(t, f.Selection.Rows, "a cycle never keeps itself alive")
}

func TestAutoParents_DirectCandidateParentStaysDeclarable(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	putChain(f)
	addTrigger(f, "t1", "a1")
	// The person is also a trigger object of another decision of the case.
	f.Decisions.Add(f.OrgId, "persons", map[string]any{"object_id": "p1", "company_id": "c1"}, time.Now())

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	byKey := map[string]models.SarSelectedObject{}
	for _, r := range f.Selection.Rows {
		byKey[r.TableName+"/"+r.ObjectId] = r
	}
	p := byKey["persons/p1"]
	assert.True(t, p.AutoAdded, "still shown as added automatically")
	assert.False(t, p.ContextOnly, "a direct candidate can be declared")
	assert.Equal(t, models.SarOriginKindDecision, p.Origin.Kind, "snapshot like an analyst choice")
	assert.True(t, byKey["accounts/a1"].ContextOnly)
	assert.True(t, byKey["companies/c1"].ContextOnly)
	assert.False(t, byKey["transactions/t1"].AutoAdded)
}

func TestAutoParents_ExplicitAdditionOfAnAutoRowFollowsItsParents(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	putChain(f)
	addTrigger(f, "t1", "a1")
	st, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	require.NotContains(t, selectedTables(f), "groups/g1", "4th hop")

	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, []SelectionAdd{{Ref: selRef("companies", "c1")}}, nil)
	require.NoError(t, err)
	got := selectedTables(f)
	assert.Equal(t, false, got["companies/c1"])
	assert.Contains(t, got, "groups/g1", "traversal restarts from the converted row")
	for _, r := range f.Selection.Rows {
		if r.TableName == "companies" {
			assert.False(t, r.ContextOnly)
		}
	}
}

func TestAutoParents_ReadsAreBounded(t *testing.T) {
	uc, f, declId := selSetup(t)
	parentsModel(f)
	acc := f.DataModel.Model.Tables["accounts"]
	acc.SemanticType = models.SemanticTypeUnset // never added: nothing stops the traversal but the budget
	f.DataModel.Model.Tables["accounts"] = acc
	now := time.Now()
	var adds []SelectionAdd
	n := maxParentReads + 20
	for i := 0; i < n; i++ {
		a, id := fmt.Sprintf("a%04d", i), fmt.Sprintf("t%04d", i)
		f.Ingested.Put("accounts", a, map[string]any{"object_id": a}, now)
		addTrigger(f, id, a)
		adds = append(adds, SelectionAdd{Ref: selRef("transactions", id)})
	}
	before := f.Ingested.Reads
	_, err := uc.UpdateSelection(f.Ctx, declId, 1, adds, nil)
	require.NoError(t, err)
	assert.LessOrEqual(t, f.Ingested.Reads-before, maxParentReads)
}
