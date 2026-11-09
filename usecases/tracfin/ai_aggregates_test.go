package tracfin

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

const (
	aggTotal = "/ds/operations/montantTotal"
	aggStart = "/ds/operations/dateDebut"
	aggEnd   = "/ds/operations/dateFin"
	aggLine  = "/ds/operations/operation/montantTotal"
)

func aggOperation(id, amount string, evidence ...models.TracfinAIEvidence) models.TracfinBlock {
	b := task5Block(id, "/ds/operations/operation")
	if amount != "" {
		d := models.TracfinDecimal(amount)
		b.Fields[aggLine] = []models.TracfinValue{{ID: id + "-amount", Kind: "decimal", Decimal: &d, Origin: &models.TracfinOrigin{Source: "ai", Resolution: "resolved", Evidence: evidence}}}
	}
	return b
}

func aggFixture(c Catalog, operations ...models.TracfinBlock) (models.TracfinDraft, []models.TracfinAISource) {
	d := task5Draft(c, task5Block("ops", "/ds/operations", operations...))
	snapshot := func(field string) models.TracfinMappingSource {
		return models.TracfinMappingSource{Kind: "decision_snapshot", Field: field, HistoricalType: "poc_tracfin_transfer"}
	}
	d.Configuration.Mappings = []models.TracfinMapping{
		{ID: "start", TargetField: aggStart, Source: snapshot("creation_date")},
		{ID: "end", TargetField: aggEnd, Source: snapshot("creation_date")},
		{ID: "line", TargetField: aggLine, Source: snapshot("amount"), Unit: "major", TargetCurrency: "EUR", Currency: models.TracfinCurrencyEvidence{Field: "currency"}},
		{ID: "total", TargetField: aggTotal, Source: snapshot("amount"), Unit: "major", TargetCurrency: "EUR", Currency: models.TracfinCurrencyEvidence{Field: "currency"}},
		{ID: "count", TargetField: "/ds/operations/operation/nombreOperations", Source: models.TracfinMappingSource{Kind: "decision_snapshot", Field: "object_id", UnitOperation: true, HistoricalType: "poc_tracfin_transfer"}},
	}
	sources := []models.TracfinAISource{
		{ID: "object:poc_tracfin_transfer:R1", Kind: "ingested_object", Content: json.RawMessage(`{"id":"R1","table":"poc_tracfin_transfer","object":{"Data":{"amount":9900,"creation_date":"2026-11-01T08:00:00Z"}}}`)},
		// 23:30 UTC on 05/11 is already 06/11 in Paris.
		{ID: "snapshot:D2", Kind: "decision_snapshot", Content: json.RawMessage(`{"historical_type":"poc_tracfin_transfer","snapshot":{"object_id":"R2","amount":9900,"creation_date":"2026-11-05T23:30:00Z"}}`)},
		{ID: "object:account:A1", Kind: "ingested_object", Content: json.RawMessage(`{"id":"A1","table":"account","object":{"Data":{"creation_date":"2020-01-01T00:00:00Z"}}}`)},
	}
	return d, sources
}

func aggValue(d models.TracfinDraft, field string) *models.TracfinValue {
	var out *models.TracfinValue
	WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if b.CatalogID == "/ds/operations" && len(b.Fields[field]) == 1 {
			v := b.Fields[field][0]
			out = &v
		}
	})
	return out
}

func TestComputeAIOperationAggregates(t *testing.T) {
	c := task5Catalog(t)
	d, sources := aggFixture(c,
		aggOperation("op1", "9900", models.TracfinAIEvidence{SourceID: "object:poc_tracfin_transfer:R1", Locator: "/object/Data/amount"}, models.TracfinAIEvidence{SourceID: "object:account:A1", Locator: "/object/Data/currency"}),
		aggOperation("op2", "9900.50", models.TracfinAIEvidence{SourceID: "snapshot:D2", Locator: "/snapshot/amount"}),
	)
	out, ds := ComputeAIOperationAggregates(d, c, sources, "run")
	require.Empty(t, ds)
	total := aggValue(out, aggTotal)
	require.NotNil(t, total)
	require.Equal(t, "19800.50", string(*total.Decimal))
	require.Equal(t, "calculated", total.Origin.Source)
	require.Contains(t, total.Origin.Explanation, "EUR")
	require.NotEmpty(t, total.Origin.Evidence)
	require.Equal(t, "01/11/2026", *aggValue(out, aggStart).Text, "the account creation date is not an operation date")
	require.Equal(t, "06/11/2026", *aggValue(out, aggEnd).Text)
	require.False(t, aggValue(out, aggEnd).NeedsReview)

	again, ds := ComputeAIOperationAggregates(out, c, sources, "run")
	require.Empty(t, ds)
	require.Equal(t, "19800.50", string(*aggValue(again, aggTotal).Decimal), "recomputing replaces, never duplicates")
}

func TestComputeAIOperationAggregatesNeverGuesses(t *testing.T) {
	c := task5Catalog(t)
	d, sources := aggFixture(c,
		aggOperation("op1", "9900", models.TracfinAIEvidence{SourceID: "object:poc_tracfin_transfer:R1", Locator: "/object/Data/amount"}),
		aggOperation("op2", ""),
	)
	out, ds := ComputeAIOperationAggregates(d, c, sources, "run")
	require.Nil(t, aggValue(out, aggTotal), "a line without amount makes the total unknown")
	require.Equal(t, "01/11/2026", *aggValue(out, aggStart).Text)
	require.True(t, aggValue(out, aggStart).NeedsReview, "a line without date makes the period partial")
	codes := map[string]bool{}
	for _, x := range ds {
		require.Equal(t, "warning", x.Severity)
		codes[x.Code] = true
	}
	require.True(t, codes["AMOUNT_TOTAL_INCOMPLETE"], "%+v", ds)
	require.True(t, codes["PARTIAL_PERIOD"], "%+v", ds)

	d.Configuration.Mappings = d.Configuration.Mappings[2:3] // keep only the line amount mapping
	out, ds = ComputeAIOperationAggregates(d, c, sources, "run")
	require.Nil(t, aggValue(out, aggStart), "no configured date mapping: no period")
	require.Nil(t, aggValue(out, aggTotal), "no established aggregate currency: no total")
	require.NotEmpty(t, ds)
}

func TestComputeAIOperationAggregatesKeepsManualValues(t *testing.T) {
	c := task5Catalog(t)
	d, sources := aggFixture(c, aggOperation("op1", "9900", models.TracfinAIEvidence{SourceID: "object:poc_tracfin_transfer:R1", Locator: "/object/Data/amount"}))
	manual := "15/10/2026"
	d.Document.Root.Children[0].Fields[aggStart] = []models.TracfinValue{{ID: "m", Kind: "text", Text: &manual, Origin: &models.TracfinOrigin{Source: "manual", Resolution: "resolved"}}}
	out, _ := ComputeAIOperationAggregates(d, c, sources, "run")
	require.Equal(t, manual, *aggValue(out, aggStart).Text)
	require.Equal(t, "01/11/2026", *aggValue(out, aggEnd).Text)
}

func TestAIBackendComputedFieldsAreNotTheModelsJob(t *testing.T) {
	c := task5Catalog(t)
	text := "01/01/2026"
	p := AISectionProposal{Values: []AIValue{{Key: "start", BlockKey: "ops", FieldID: aggStart, Kind: "text", Text: &text, Explanation: "x", Sources: []AIEvidence{{SourceID: "s", Locator: "/x"}}}}}
	d := task5Draft(c, task5Block("ops", "/ds/operations", aggOperation("op1", "")))
	out, ds := ApplyAISection(d, "operations", p, map[string]bool{"s": true}, c, "run")
	for _, x := range ds {
		if x.Field != nil {
			require.NotContains(t, []string{aggStart, aggEnd, aggTotal}, x.Field.FieldID, "%+v", x)
		}
	}
	require.Nil(t, aggValue(out, aggStart), "a model value on a backend-computed field is ignored")
}

func TestComputeAIOperationCountFromCitedTransactions(t *testing.T) {
	c := task5Catalog(t)
	count := "/ds/operations/operation/nombreOperations"
	d, sources := aggFixture(c,
		// The ingested object and the decision snapshot of one transfer are one operation.
		aggOperation("same", "9900", models.TracfinAIEvidence{SourceID: "object:poc_tracfin_transfer:R1", Locator: "/object/Data/amount"}, models.TracfinAIEvidence{SourceID: "snapshot:R1", Locator: "/snapshot/amount"}, models.TracfinAIEvidence{SourceID: "object:account:A1", Locator: "/object/Data/currency"}),
		aggOperation("two", "19800", models.TracfinAIEvidence{SourceID: "object:poc_tracfin_transfer:R1", Locator: "/object/Data/amount"}, models.TracfinAIEvidence{SourceID: "snapshot:D2", Locator: "/snapshot/amount"}),
		aggOperation("none", "1", models.TracfinAIEvidence{SourceID: "object:account:A1", Locator: "/object/Data/currency"}),
	)
	d.Configuration.Mappings = append(d.Configuration.Mappings, models.TracfinMapping{ID: "count", TargetField: count, Source: models.TracfinMappingSource{Kind: "decision_snapshot", Field: "object_id", UnitOperation: true, HistoricalType: "poc_tracfin_transfer"}})
	sources = append(sources, models.TracfinAISource{ID: "snapshot:R1", Kind: "decision_snapshot", Content: json.RawMessage(`{"historical_type":"poc_tracfin_transfer","snapshot":{"object_id":"R1","amount":9900,"creation_date":"2026-11-01T08:00:00Z"}}`)})
	sources[1].Content = json.RawMessage(`{"historical_type":"poc_tracfin_transfer","snapshot":{"object_id":"R2","amount":9900,"creation_date":"2026-11-05T23:30:00Z"}}`)
	out, ds := ComputeAIOperationAggregates(d, c, sources, "run")
	got := map[string]string{}
	WalkBlocks(out.Document.Root, func(b models.TracfinBlock) {
		if v := b.Fields[count]; len(v) == 1 {
			got[b.ID] = *v[0].Text
			require.Equal(t, "calculated", v[0].Origin.Source)
		}
	})
	require.Equal(t, map[string]string{"same": "1", "two": "2"}, got)
	found := false
	for _, x := range ds {
		if x.Code == "OPERATION_COUNT_UNAVAILABLE" && x.Field.BlockID == "none" {
			found = true
		}
	}
	require.True(t, found, "%+v", ds)
}
