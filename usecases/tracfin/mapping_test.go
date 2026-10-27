package tracfin

import (
	"encoding/json"
	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTracfinNestedSelection(t *testing.T) {
	config := models.TracfinConfiguration{Mappings: []models.TracfinMapping{
		{ID: "user", TargetField: "/ds/declarant/coordonnees/nom", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "name"}},
		{ID: "account", TargetField: "/ds/declarant/coordonnees/prenom", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "number", Path: []models.TracfinPathStep{{ID: "account", ParentID: "root", Table: "Account", Link: "owner", Direction: "inverse"}}}},
		{ID: "card", TargetField: "/ds/declarant/coordonnees/tel", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "number", Path: []models.TracfinPathStep{{ID: "account", ParentID: "root", Table: "Account", Link: "owner", Direction: "inverse"}, {ID: "card", ParentID: "account", Table: "Card", Link: "account", Direction: "inverse"}}}},
	}}
	sel := models.TracfinSelection{MappingID: "user", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U1"}, Bindings: []models.TracfinPathStep{{ID: "account", ParentID: "root", Table: "Account", RecordID: "A2", Link: "owner", Direction: "inverse"}, {ID: "card", ParentID: "account", Table: "Card", RecordID: "C2", Link: "account", Direction: "inverse"}}}
	draft := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "person", CatalogID: "/ds/declarant/coordonnees", Selection: &sel}}}
	sources := SourceSet{Records: map[SourceKey]SourceRecord{
		{"person", "root"}:    {Table: "User", RecordID: "U1", Data: map[string]any{"name": "Alice"}},
		{"person", "account"}: {Table: "Account", RecordID: "A2", Data: map[string]any{"number": "A2-number"}},
		{"person", "card"}:    {Table: "Card", RecordID: "C2", Data: map[string]any{"number": "C2-number"}},
		{"other", "account"}:  {Table: "Account", RecordID: "A1", Data: map[string]any{"number": "WRONG"}},
	}}
	got := ResolvePrefill(config, draft, sources)
	require.Len(t, got.Proposals, 3)
	require.Equal(t, "A2-number", *got.Proposals[1].Candidates[0].Text)
	require.Equal(t, "C2-number", *got.Proposals[2].Candidates[0].Text)
	delete(sources.Records, SourceKey{"person", "card"})
	require.Len(t, ResolvePrefill(config, draft, sources).Proposals, 2)
}
func TestTracfinDecisionWithoutIngestion(t *testing.T) {
	raw := json.RawMessage(`{"object_id":9007199254740993,"amount":10001,"currency":"EUR"}`)
	data, err := DecodeSnapshot(raw)
	require.NoError(t, err)
	require.Equal(t, json.Number("9007199254740993"), data["object_id"])
	config := models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "operation", TargetField: "/ds/operations/operation/montantTotal", Unit: "minor", TargetCurrency: "EUR", Currency: models.TracfinCurrencyEvidence{Field: "currency"}, Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "DeletedTransaction", Field: "amount"}}}}
	draft := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "op", CatalogID: "/ds/operations/operation", Selection: &models.TracfinSelection{MappingID: "operation", DecisionID: "D1", HistoricalType: "DeletedTransaction"}}}}
	sources := SourceSet{Decisions: map[string]models.TracfinDecisionSource{"D1": {DecisionID: "D1", HistoricalType: "DeletedTransaction", Snapshot: raw}}}
	got := ResolvePrefill(config, draft, sources)
	require.Len(t, got.Proposals, 1)
	require.Equal(t, models.TracfinDecimal("100.01"), *got.Proposals[0].Candidates[0].Decimal)
	config.Mappings[0].Currency.Constant = "USD"
	config.Mappings[0].Currency.Field = ""
	got = ResolvePrefill(config, draft, sources)
	require.Empty(t, got.Proposals)
	require.Equal(t, "CURRENCY_MISMATCH", got.Diagnostics[0].Code)
}
func TestTracfinPrefillConcurrentSources(t *testing.T) {
	for _, different := range []bool{false, true} {
		t.Run(map[bool]string{false: "equal", true: "conflict"}[different], func(t *testing.T) {
			mappings := []models.TracfinMapping{{ID: "one", TargetField: "/ds/declarant/coordonnees/nom", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "a"}}, {ID: "two", TargetField: "/ds/declarant/coordonnees/nom", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "b"}}}
			b := "same"
			if different {
				b = "other"
			}
			d := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "p", CatalogID: "/ds/declarant/coordonnees", Selection: &models.TracfinSelection{MappingID: "one", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}}}}}
			got := ResolvePrefill(models.TracfinConfiguration{Mappings: mappings}, d, SourceSet{Records: map[SourceKey]SourceRecord{{"p", "root"}: {Table: "User", RecordID: "U", Data: map[string]any{"a": "same", "b": b}}}})
			require.Len(t, got.Proposals, 1)
			if different {
				require.Len(t, got.Proposals[0].Candidates, 2)
				require.Equal(t, "conflict", got.Proposals[0].Resolution)
			} else {
				require.Len(t, got.Proposals[0].Candidates, 1)
				require.Len(t, got.Proposals[0].Candidates[0].Origin.Sources, 2)
			}
		})
	}
}

func TestTracfinPrefillUnknownCodesDiscriminatorAndUnits(t *testing.T) {
	field := "/ds/operations/operation/nombreOperations"
	m := models.TracfinMapping{ID: "m", TargetField: field, Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "Removed", Field: "count"}}
	d := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "o", CatalogID: "/ds/operations/operation", Selection: &models.TracfinSelection{MappingID: "m", DecisionID: "D", HistoricalType: "Removed"}}}}
	sources := SourceSet{Decisions: map[string]models.TracfinDecisionSource{"D": {DecisionID: "D", HistoricalType: "Removed", Snapshot: json.RawMessage(`{"count":"customer-code","type":"unknown"}`)}}}
	m.Codes = []models.TracfinCodeMapping{{Source: "known", Target: "1"}}
	got := ResolvePrefill(models.TracfinConfiguration{Mappings: []models.TracfinMapping{m}}, d, sources)
	require.Empty(t, got.Proposals)
	require.Equal(t, "UNKNOWN_CODE", got.Diagnostics[0].Code)
	m.Codes = nil
	m.Discriminator = &models.TracfinDiscriminator{Field: "type", Values: []string{"payment"}}
	got = ResolvePrefill(models.TracfinConfiguration{Mappings: []models.TracfinMapping{m}}, d, sources)
	require.Empty(t, got.Proposals)
	require.Equal(t, "UNKNOWN_DISCRIMINATOR", got.Diagnostics[0].Code)
	m.Discriminator = nil
	m.Source.Field = "absent"
	got = ResolvePrefill(models.TracfinConfiguration{Mappings: []models.TracfinMapping{m}}, d, sources)
	require.Empty(t, got.Proposals)
	m.Source.UnitOperation = true
	got = ResolvePrefill(models.TracfinConfiguration{Mappings: []models.TracfinMapping{m}}, d, sources)
	require.Equal(t, "1", *got.Proposals[0].Candidates[0].Text)
}
func TestTracfinPrefillCatalogAndRepeatedOccurrence(t *testing.T) {
	field := "/ds/declarant/coordonnees/nom"
	text := "source"
	m := models.TracfinMapping{ID: "s", TargetField: field, Source: models.TracfinMappingSource{Kind: "settings", Field: field}}
	c := models.TracfinConfiguration{Mappings: []models.TracfinMapping{m}, Settings: models.TracfinDocument{Root: models.TracfinBlock{Fields: map[string][]models.TracfinValue{field: {{Kind: "text", Text: &text}}}}}}
	d := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "b", CatalogID: "/ds/declarant/coordonnees"}}}
	got := ResolvePrefill(c, d, SourceSet{})
	require.Len(t, got.Proposals, 1)
	d.Document.Root.Fields = map[string][]models.TracfinValue{field: {{ID: "v", Kind: "text", Text: &text, Origin: &models.TracfinOrigin{Source: "settings"}}}}
	got = ResolvePrefill(c, d, SourceSet{})
	require.Equal(t, "v", got.Proposals[0].Field.ValueID)
	d.Document.Root.Fields[field] = append(d.Document.Root.Fields[field], models.TracfinValue{ID: "second", Origin: &models.TracfinOrigin{Source: "settings"}})
	got = ResolvePrefill(c, d, SourceSet{})
	require.Empty(t, got.Proposals)
	require.Equal(t, "TARGET_OCCURRENCE_REQUIRED", got.Diagnostics[0].Code)
	d.Document.Root.Fields[field] = []models.TracfinValue{{ID: "v", Kind: "cleared"}}
	require.Empty(t, ResolvePrefill(c, d, SourceSet{}).Proposals)
	c.Mappings[0].TargetField = "/ds/not-a-field"
	d.Document.Root.Fields = nil
	got = ResolvePrefill(c, d, SourceSet{})
	require.Empty(t, got.Proposals)
	require.Equal(t, "INVALID_MAPPING_TARGET", got.Diagnostics[0].Code)
}

func TestTracfinMappingExplicitConfiguration(t *testing.T) {
	catalog, err := LoadCatalog()
	require.NoError(t, err)
	base := models.TracfinMapping{ID: "m", TargetField: "/ds/operations/operation/nombreOperations", Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "Historic", Field: "count"}}
	for _, mutate := range []func(*models.TracfinMapping){
		func(m *models.TracfinMapping) {
			m.Source.UnitOperation = true
			m.Source.Kind = "ingested"
			m.Source.Table = "User"
			m.Source.EntityType = "Person"
		},
		func(m *models.TracfinMapping) { m.Discriminator = &models.TracfinDiscriminator{Field: "type"} },
		func(m *models.TracfinMapping) {
			m.Source.Path = []models.TracfinPathStep{{ID: "bad", ParentID: "missing", Table: "X", Link: "link", Direction: "inverse"}}
		},
	} {
		m := base
		mutate(&m)
		require.NotEmpty(t, ValidateConfiguration(models.TracfinConfiguration{Mappings: []models.TracfinMapping{m}}, catalog))
	}
}

func TestTracfinPrefillDeduplicatesProvenance(t *testing.T) {
	field := "/ds/declarant/coordonnees/nom"
	source := models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "name"}
	c := models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "one", TargetField: field, Source: source}, {ID: "two", TargetField: field, Source: source}}}
	d := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "b", CatalogID: "/ds/declarant/coordonnees", Selection: &models.TracfinSelection{MappingID: "one", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}}}}}
	got := ResolvePrefill(c, d, SourceSet{Records: map[SourceKey]SourceRecord{{"b", "root"}: {Table: "User", RecordID: "U", Data: map[string]any{"name": "value"}}}})
	require.Len(t, got.Proposals, 1)
	require.Len(t, got.Proposals[0].Candidates[0].Origin.Sources, 1)
	require.NotEmpty(t, got.Proposals[0].Candidates[0].ID)
}

func TestTracfinPrefillEquivalentMonetaryAmounts(t *testing.T) {
	for _, tc := range []struct {
		name, minor, major string
		conflict           bool
	}{
		{"scale", "10001", "100.010", false},
		{"exponent", "10001", "1.00010e2", false},
		{"signed zero", "0", "-0.000e3", false},
		{"different", "10001", "100.02", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := "/ds/operations/operation/montantTotal"
			config := models.TracfinConfiguration{Mappings: []models.TracfinMapping{
				{ID: "minor", TargetField: field, Unit: "minor", TargetCurrency: "EUR", Currency: models.TracfinCurrencyEvidence{Constant: "EUR"}, Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "Historic", Field: "minor"}},
				{ID: "major", TargetField: field, Unit: "major", TargetCurrency: "EUR", Currency: models.TracfinCurrencyEvidence{Constant: "EUR"}, Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "Historic", Field: "major"}},
			}}
			draft := models.TracfinDraft{Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "op", CatalogID: "/ds/operations/operation", Selection: &models.TracfinSelection{MappingID: "minor", DecisionID: "D", HistoricalType: "Historic"}}}}
			sources := SourceSet{Decisions: map[string]models.TracfinDecisionSource{"D": {DecisionID: "D", HistoricalType: "Historic", Snapshot: json.RawMessage(`{"minor":` + tc.minor + `,"major":` + tc.major + `}`)}}}
			got := ResolvePrefill(config, draft, sources)
			require.Len(t, got.Proposals, 1)
			if tc.conflict {
				require.Len(t, got.Proposals[0].Candidates, 2)
				require.Equal(t, "conflict", got.Proposals[0].Resolution)
			} else {
				require.Len(t, got.Proposals[0].Candidates, 1)
				require.NotEqual(t, "conflict", got.Proposals[0].Resolution)
				require.Len(t, got.Proposals[0].Candidates[0].Origin.Sources, 2)
			}
		})
	}
}
