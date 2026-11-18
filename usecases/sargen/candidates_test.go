package sargen

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// recordedRead is one call of ReadIngestedClientObjects.
type recordedRead struct {
	objectType string
	input      models.ClientDataListRequestBody
}

// recordingIngested records the ingested list reads and delegates to the fake.
type recordingIngested struct {
	IngestedDataReader
	mu    sync.Mutex
	calls []recordedRead
}

func (r *recordingIngested) ReadIngestedClientObjects(ctx context.Context, orgId uuid.UUID, objectType string,
	input models.ClientDataListRequestBody, fieldsToRead ...string,
) ([]models.ClientObjectDetail, []models.FieldStatistics, models.ClientDataListPagination, error) {
	r.mu.Lock()
	r.calls = append(r.calls, recordedRead{objectType: objectType, input: input})
	r.mu.Unlock()
	return r.IngestedDataReader.ReadIngestedClientObjects(ctx, orgId, objectType, input, fieldsToRead...)
}

// ---- ListCandidates ----

func TestListCandidates_AllDecisionsNotOnlyFive(t *testing.T) {
	uc, f, declId := selSetup(t)
	base := time.Now()
	for i := 1; i <= 7; i++ {
		f.Decisions.Add(f.OrgId, "transactions",
			map[string]any{"object_id": fmt.Sprintf("t%d", i)}, base.Add(time.Duration(i)*time.Minute))
	}
	cands, err := uc.ListCandidates(f.Ctx, declId)
	require.NoError(t, err)

	require.Len(t, cands, 7)
	seen := map[core.ObjectRef]bool{}
	for _, c := range cands {
		assert.Equal(t, "transactions", c.Ref.Table)
		assert.Equal(t, ReasonTriggerObject, c.Reason)
		seen[c.Ref] = true
	}
	for i := 1; i <= 7; i++ {
		assert.True(t, seen[selRef("transactions", fmt.Sprintf("t%d", i))], "t%d must be a candidate", i)
	}
	assert.Equal(t, 1, f.Decisions.LightCalls, "all the decisions of the case come from one light query")
}

func TestListCandidates_DeduplicatesAcrossReasons(t *testing.T) {
	uc, f, declId := selSetup(t)
	// t1 is the trigger object of a decision, a pivot object and a case entity.
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1"}, time.Now())
	f.Ingested.Put("transactions", "t1", map[string]any{"object_id": "t1"}, time.Now())
	f.CaseEntities.Refs = []models.CaseEntityRef{{TableName: "transactions", ObjectId: "t1"}}
	f.Ingested.Pivots = []models.PivotObject{
		{
			PivotObjectName: "transactions", PivotObjectId: "t1", IsIngested: true,
			PivotObjectData: models.ClientObjectDetail{Data: map[string]any{"object_id": "t1"}},
		},
		{
			PivotObjectName: "accounts", PivotObjectId: "a1", IsIngested: true,
			PivotObjectData: models.ClientObjectDetail{Data: map[string]any{"object_id": "a1"}},
		},
	}

	cands, err := uc.ListCandidates(f.Ctx, declId)
	require.NoError(t, err)

	counts := map[core.ObjectRef]int{}
	for _, c := range cands {
		counts[c.Ref]++
	}
	for ref, n := range counts {
		assert.Equal(t, 1, n, "%v must be listed once", ref)
	}
	assert.Len(t, cands, 2)
	assert.Equal(t, 1, counts[selRef("transactions", "t1")])
	assert.Equal(t, 1, counts[selRef("accounts", "a1")])
}

func TestListCandidates_OtherOrgDeclarationForbidden(t *testing.T) {
	uc, f, declId := selSetup(t)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1"}, time.Now())

	_, err := uc.ListCandidates(selOtherOrgCtx(), declId)
	require.Error(t, err)
	assert.ErrorIs(t, err, models.ForbiddenError)
}

// ---- Navigate ----

func TestNavigate_SourceNotReachableIsForbidden(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a1"}, time.Now())
	// Reading the candidate set is expected; nothing about the unreachable a9
	// (neither a candidate nor selected) may be read.
	f.Ingested.Put("accounts", "a9", map[string]any{"object_id": "a9"}, time.Now())

	_, _, err := uc.Navigate(f.Ctx, declId, selRef("accounts", "a9"), "account", nil, 10)
	require.Error(t, err)
	assert.ErrorIs(t, err, models.ForbiddenError)
	assert.Equal(t, 0, f.Ingested.Reads, "nothing must be read from an unreachable source")
}

func TestNavigate_ToManyUsesNavigationOption(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	selAddCaseEntity(f, "accounts", "a1", map[string]any{"object_id": "a1"})
	selAddCaseEntity(f, "accounts", "a2", map[string]any{"object_id": "a2"})
	f.Ingested.Put("transactions", "t1", map[string]any{"object_id": "t1", "account_id": "a1"}, time.Now())
	f.Ingested.Put("transactions", "t2", map[string]any{"object_id": "t2", "account_id": "a1"}, time.Now())
	f.Ingested.Put("transactions", "t3", map[string]any{"object_id": "t3", "account_id": "a2"}, time.Now())

	cands, err := uc.ListCandidates(f.Ctx, declId)
	require.NoError(t, err)
	var link *LinkOption
	for _, c := range cands {
		if c.Ref != selRef("accounts", "a1") {
			continue
		}
		for i := range c.Links {
			if c.Links[i].Kind == LinkKindToMany && c.Links[i].TargetTable == "transactions" {
				link = &c.Links[i]
			}
		}
	}
	require.NotNil(t, link, "accounts a1 must offer the to-many link towards transactions")

	rec := &recordingIngested{IngestedDataReader: f.Ingested}
	uc.ingestedReader = rec

	// Asking for 500 must be capped to MaxNavigateLimit (100).
	got, _, err := uc.Navigate(f.Ctx, declId, selRef("accounts", "a1"), link.Name, nil, 500)
	require.NoError(t, err)

	var txReads []recordedRead
	for _, c := range rec.calls {
		if c.objectType == "transactions" {
			txReads = append(txReads, c)
		}
	}
	require.NotEmpty(t, txReads, "the to-many navigation must read the target table")
	read := txReads[len(txReads)-1]
	ex := read.input.ExplorationOptions
	assert.Equal(t, "accounts", ex.SourceTableName)
	assert.Equal(t, "account_id", ex.FilterFieldName)
	require.NotNil(t, ex.FilterFieldValue.StringValue)
	assert.Equal(t, "a1", *ex.FilterFieldValue.StringValue)
	assert.Equal(t, "object_id", ex.OrderingFieldName)
	assert.Equal(t, MaxNavigateLimit, read.input.Limit)
	assert.LessOrEqual(t, read.input.Limit, 100)

	require.Len(t, got, 2)
	for _, c := range got {
		assert.Equal(t, "transactions", c.Ref.Table)
	}
}

func TestNavigate_ToSingle(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a1"}, time.Now())
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t2", "account_id": "zz"}, time.Now().Add(-time.Hour))
	f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1"}, time.Now())

	got, next, err := uc.Navigate(f.Ctx, declId, selRef("transactions", "t1"), "account", nil, 10)
	require.NoError(t, err)
	assert.Nil(t, next)
	require.Len(t, got, 1)
	assert.Equal(t, selRef("accounts", "a1"), got[0].Ref)

	// Parent missing from the ingested data: empty result, no error.
	got, _, err = uc.Navigate(f.Ctx, declId, selRef("transactions", "t2"), "account", nil, 10)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestNavigate_ToManyCursorPassthrough(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	selAddCaseEntity(f, "accounts", "a1", map[string]any{"object_id": "a1"})
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("t%d", i)
		f.Ingested.Put("transactions", id, map[string]any{"object_id": id, "account_id": "a1"}, time.Now())
	}
	from := selRef("accounts", "a1")

	page1, next, err := uc.Navigate(f.Ctx, declId, from, "transactions.account_id", nil, 2)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.NotNil(t, next)

	page2, next, err := uc.Navigate(f.Ctx, declId, from, "transactions.account_id", next, 2)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.Equal(t, selRef("transactions", "t3"), page2[0].Ref)
	assert.Nil(t, next)
}

func TestNavigate_FromSelectedObjectNoLongerCandidate(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	selAddCaseEntity(f, "accounts", "a1", map[string]any{"object_id": "a1"})
	f.Ingested.Put("transactions", "t1", map[string]any{"object_id": "t1", "account_id": "a1"}, time.Now())
	from := selRef("accounts", "a1")
	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: from}}, nil)
	require.NoError(t, err)

	f.CaseEntities.Refs = nil // a1 is not a candidate anymore, only selected

	got, _, err := uc.Navigate(f.Ctx, declId, from, "transactions.account_id", nil, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestNavigate_OtherOrgForbidden(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a1"}, time.Now())

	_, _, err := uc.Navigate(selOtherOrgCtx(), declId, selRef("transactions", "t1"), "account", nil, 10)
	require.Error(t, err)
	assert.ErrorIs(t, err, models.ForbiddenError)
	assert.Equal(t, 0, f.Ingested.Reads)
}

func TestNavigate_FilterTypedFromFieldDataType(t *testing.T) {
	cases := []struct {
		name       string
		targetType models.DataType
		sourceVal  any
		targetVal  any
		wantFloat  bool
	}{
		{"numeric target field gets a number", models.Float, float64(42), float64(42), true},
		{"string target field gets a string even from a numeric source", models.String, float64(7), "7", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, f, declId := selSetup(t)
			f.DataModel.Model = models.DataModel{Tables: map[string]models.Table{
				"accounts": {
					Name: "accounts",
					Fields: map[string]models.Field{
						"object_id": {Name: "object_id", DataType: models.String},
						"num":       {Name: "num", DataType: tc.targetType},
					},
					NavigationOptions: []models.NavigationOption{{
						SourceTableName: "accounts", SourceFieldName: "num",
						TargetTableName: "transactions", FilterFieldName: "ref",
						OrderingFieldName: "object_id", Status: models.IndexStatusValid,
					}},
				},
				"transactions": {
					Name: "transactions",
					Fields: map[string]models.Field{
						"object_id": {Name: "object_id", DataType: models.String},
						"ref":       {Name: "ref", DataType: tc.targetType},
					},
				},
			}}
			selAddCaseEntity(f, "accounts", "a1", map[string]any{"object_id": "a1", "num": tc.sourceVal})
			f.Ingested.Put("transactions", "t1", map[string]any{"object_id": "t1", "ref": tc.targetVal}, time.Now())

			rec := &recordingIngested{IngestedDataReader: f.Ingested}
			uc.ingestedReader = rec

			got, _, err := uc.Navigate(f.Ctx, declId, selRef("accounts", "a1"), "transactions.ref", nil, 10)
			require.NoError(t, err, "the fake reader rejects a mistyped filter like the real one")
			require.Len(t, got, 1)

			require.NotEmpty(t, rec.calls)
			fv := rec.calls[len(rec.calls)-1].input.ExplorationOptions.FilterFieldValue
			if tc.wantFloat {
				require.NotNil(t, fv.FloatValue)
				assert.Nil(t, fv.StringValue)
			} else {
				require.NotNil(t, fv.StringValue)
				assert.Nil(t, fv.FloatValue)
			}
		})
	}
}

// Navigation only needs the trigger objects of the case: it must not run the
// heavy decision loader that also reads the rules and screenings of every
// decision (D2).
func TestNavigate_DoesNotLoadRulesOfEveryCaseDecision(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	base := time.Now().Add(-time.Hour)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a1"}, base)
	for i := 2; i <= 4; i++ {
		f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": fmt.Sprintf("t%d", i)}, base)
	}
	f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1"}, base)

	got, _, err := uc.Navigate(f.Ctx, declId, selRef("transactions", "t1"), "account", nil, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Positive(t, f.Decisions.LightCalls, "navigation reads the case decisions through the light reader")
}

// A selected object is followed from its frozen copy, not from the live client
// data: a newer decision on t1 that now points to a2 must not change the
// navigation from the a1 the analyst selected (D3).
func TestNavigate_SelectedObjectUsesFrozenCopy(t *testing.T) {
	uc, f, declId := selSetup(t)
	selDataModel(f)
	base := time.Now().Add(-time.Hour)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a1"}, base)
	f.Ingested.Put("accounts", "a1", map[string]any{"object_id": "a1", "name": "Acme"}, base)
	f.Ingested.Put("accounts", "a2", map[string]any{"object_id": "a2", "name": "Autre"}, base)

	_, err := uc.UpdateSelection(f.Ctx, declId, 1, []SelectionAdd{{Ref: selRef("transactions", "t1")}}, nil)
	require.NoError(t, err)
	f.Decisions.Add(f.OrgId, "transactions", map[string]any{"object_id": "t1", "account_id": "a2"}, time.Now())

	got, _, err := uc.Navigate(f.Ctx, declId, selRef("transactions", "t1"), "account", nil, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a1", got[0].Ref.ObjectID, "the link must be followed from the frozen copy (a1), not the live data (a2)")
}
