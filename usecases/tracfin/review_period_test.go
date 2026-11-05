package tracfin

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

// The declaration period is proposed from the known dates of the selected
// principal operations; lines without a usable date make it partial.
func TestTracfinPrefillOperationPeriod(t *testing.T) {
	period := func(target string) models.TracfinMapping {
		return models.TracfinMapping{ID: target, TargetField: target, Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "Transfer", Field: "date"}}
	}
	config := models.TracfinConfiguration{Mappings: []models.TracfinMapping{period("/ds/operations/dateDebut"), period("/ds/operations/dateFin")}}
	ops := models.TracfinBlock{ID: "ops", CatalogID: "/ds/operations"}
	decisions := map[string]models.TracfinDecisionSource{}
	for id, snapshot := range map[string]string{"D1": `{"date":"2026-03-02T10:00:00Z"}`, "D2": `{"date":"2026-01-15"}`, "D3": `{"amount":1}`} {
		decisions[id] = models.TracfinDecisionSource{DecisionID: id, HistoricalType: "Transfer", Snapshot: json.RawMessage(snapshot)}
	}
	for _, id := range []string{"D1", "D2", "D3"} {
		ops.Children = append(ops.Children, models.TracfinBlock{ID: "op-" + id, CatalogID: "/ds/operations/operation", Selection: &models.TracfinSelection{MappingID: "/ds/operations/dateDebut", DecisionID: id, HistoricalType: "Transfer"}})
	}
	draft := models.TracfinDraft{Revision: 3, Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "root", CatalogID: "/ds", Children: []models.TracfinBlock{ops}}}}

	got := ResolvePrefill(config, draft, SourceSet{Decisions: decisions})
	byField := map[string]PrefillProposal{}
	for _, p := range got.Proposals {
		byField[p.Field.FieldID] = p
	}
	require.Equal(t, "15/01/2026", *byField["/ds/operations/dateDebut"].Candidates[0].Text)
	require.Equal(t, "02/03/2026", *byField["/ds/operations/dateFin"].Candidates[0].Text)
	for _, p := range byField {
		require.Equal(t, "ops", p.Field.BlockID)
		origin := p.Candidates[0].Origin
		require.Equal(t, "calculated", origin.Source)
		var used []string
		for _, s := range origin.Sources {
			used = append(used, s.DecisionID)
		}
		require.ElementsMatch(t, []string{"D1", "D2"}, used)
	}
	partial := 0
	for _, d := range got.Diagnostics {
		if d.Code == "PARTIAL_PERIOD" {
			partial++
			require.Equal(t, "ops", d.Field.BlockID)
		}
	}
	require.Equal(t, 2, partial, "the missing date of D3 is reported for both bounds")
	for _, p := range byField {
		require.Equal(t, "partial", p.Resolution, "a partial period is not presented as resolved")
		require.True(t, p.Candidates[0].NeedsReview)
	}

	// A manual period is never replaced by the calculated one.
	manual, err := ApplyChanges(draft, []Change{draftChange("set_field", "ops", "/ds/operations/dateDebut", "", textInput("01/01/2026"))}, draftCatalog(t))
	require.NoError(t, err)
	got = ResolvePrefill(config, manual, SourceSet{Decisions: decisions})
	for _, p := range got.Proposals {
		require.NotEqual(t, "/ds/operations/dateDebut", p.Field.FieldID)
	}
}
