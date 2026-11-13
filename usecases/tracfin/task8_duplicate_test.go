package tracfin

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

func task8OperationDraft(selections ...models.TracfinSelection) models.TracfinDraft {
	ops := models.TracfinBlock{ID: "ops", CatalogID: "/ds/operations"}
	for _, s := range selections {
		ops.Children = append(ops.Children, models.TracfinBlock{ID: "op-" + s.DecisionID, CatalogID: "/ds/operations/operation", Selection: &s})
	}
	return models.TracfinDraft{Revision: 2, Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "root", CatalogID: "/ds", Children: []models.TracfinBlock{ops}}}}
}

func task8Duplicates(ds []models.TracfinDiagnostic) map[string][]string {
	out := map[string][]string{}
	for _, d := range ds {
		if d.Code != "POTENTIAL_DUPLICATE_OPERATION" {
			continue
		}
		for _, s := range d.Sources {
			out[d.Field.BlockID] = append(out[d.Field.BlockID], s.DecisionID)
		}
	}
	return out
}

func TestTracfinDecisionSnapshotDuplicateWarning(t *testing.T) {
	config := models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "amount", TargetField: "/ds/operations/operation/montantTotal", Unit: "minor", TargetCurrency: "EUR", Currency: models.TracfinCurrencyEvidence{Constant: "EUR"}, Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "Payment", Field: "amount"}}}}
	selection := func(id, historicalType string) models.TracfinSelection {
		return models.TracfinSelection{MappingID: "amount", DecisionID: id, HistoricalType: historicalType}
	}
	snapshot := func(id, historicalType, raw string) models.TracfinDecisionSource {
		return models.TracfinDecisionSource{DecisionID: id, HistoricalType: historicalType, Snapshot: json.RawMessage(raw)}
	}

	t.Run("same historical type and object_id", func(t *testing.T) {
		draft := task8OperationDraft(selection("D1", "Payment"), selection("D2", "Payment"))
		got := ResolvePrefill(config, draft, SourceSet{Decisions: map[string]models.TracfinDecisionSource{
			"D1": snapshot("D1", "Payment", `{"object_id":"T1","amount":1000}`),
			"D2": snapshot("D2", "Payment", `{"object_id":"T1","amount":2500}`),
		}})
		require.Equal(t, map[string][]string{"op-D1": {"D1", "D2"}, "op-D2": {"D1", "D2"}}, task8Duplicates(got.Diagnostics))
		for _, d := range got.Diagnostics {
			if d.Code == "POTENTIAL_DUPLICATE_OPERATION" {
				require.Equal(t, "warning", d.Severity)
				require.Equal(t, "sources", d.Layer)
			}
		}
		// Each snapshot keeps its own proposal: no merge and no automatic sum.
		amounts := map[string]models.TracfinDecimal{}
		for _, p := range got.Proposals {
			require.Len(t, p.Candidates, 1)
			amounts[p.Field.BlockID] = *p.Candidates[0].Decimal
		}
		require.Equal(t, map[string]models.TracfinDecimal{"op-D1": "10.00", "op-D2": "25.00"}, amounts)
	})

	for name, tc := range map[string]struct {
		types     [2]string
		snapshots [2]string
	}{
		"different historical type":                  {[2]string{"Payment", "Refund"}, [2]string{`{"object_id":"T1"}`, `{"object_id":"T1"}`}},
		"different object_id":                        {[2]string{"Payment", "Payment"}, [2]string{`{"object_id":"T1"}`, `{"object_id":"T2"}`}},
		"same name or business id without object_id": {[2]string{"Payment", "Payment"}, [2]string{`{"id":"X","name":"Same","transaction_id":"T"}`, `{"id":"X","name":"Same","transaction_id":"T"}`}},
		"same name with distinct object_id":          {[2]string{"Payment", "Payment"}, [2]string{`{"object_id":"T1","name":"Same"}`, `{"object_id":"T2","name":"Same"}`}},
		"empty object_id":                            {[2]string{"Payment", "Payment"}, [2]string{`{"object_id":""}`, `{"object_id":""}`}},
		"large numeric ids differ exactly":           {[2]string{"Payment", "Payment"}, [2]string{`{"object_id":9007199254740993}`, `{"object_id":9007199254740992}`}},
	} {
		t.Run(name, func(t *testing.T) {
			draft := task8OperationDraft(selection("D1", tc.types[0]), selection("D2", tc.types[1]))
			got := ResolvePrefill(config, draft, SourceSet{Decisions: map[string]models.TracfinDecisionSource{
				"D1": snapshot("D1", tc.types[0], tc.snapshots[0]),
				"D2": snapshot("D2", tc.types[1], tc.snapshots[1]),
			}})
			require.Empty(t, task8Duplicates(got.Diagnostics))
		})
	}

	t.Run("equal large numeric object_id", func(t *testing.T) {
		draft := task8OperationDraft(selection("D1", "Payment"), selection("D2", "Payment"))
		got := ResolvePrefill(config, draft, SourceSet{Decisions: map[string]models.TracfinDecisionSource{
			"D1": snapshot("D1", "Payment", `{"object_id":9007199254740993}`),
			"D2": snapshot("D2", "Payment", `{"object_id":9007199254740993}`),
		}})
		require.Len(t, task8Duplicates(got.Diagnostics), 2)
	})
}
