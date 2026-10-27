package tracfin

import (
	"math"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

// The client database returns native pgx values: int32 for INTEGER, float64
// for FLOAT and time.Time for TIMESTAMPTZ, not strings.
func TestTracfinPrefillNativeIngestedTypes(t *testing.T) {
	const civil = "/ds/personnes/pp/etatCivil"
	const account = "/ds/personnes/pp/referencesFinancieres/referenceFinanciereIdentifiantCompte"
	mapping := func(id, target, field string) models.TracfinMapping {
		return models.TracfinMapping{ID: id, TargetField: target, Source: models.TracfinMappingSource{Kind: "ingested", EntityType: "Person", Table: "User", Field: field}}
	}
	balance := mapping("balance", account+"/@soldeRestant", "balance")
	balance.Unit, balance.TargetCurrency, balance.Currency = "major", "EUR", models.TracfinCurrencyEvidence{Constant: "EUR"}
	cents := mapping("cents", account+"/@soldeRestant", "cents")
	cents.Unit, cents.TargetCurrency, cents.Currency = "minor", "EUR", models.TracfinCurrencyEvidence{Constant: "EUR"}
	selection := &models.TracfinSelection{MappingID: "birth", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}}

	for name, tc := range map[string]struct {
		mapping models.TracfinMapping
		block   string
		value   any
		want    string
		code    string
	}{
		"timestamp date":            {mapping("birth", civil+"/dateNaissance", "v"), civil, time.Date(1985, 7, 14, 0, 0, 0, 0, time.UTC), "14/07/1985", ""},
		"timestamp in another zone": {mapping("birth", civil+"/dateNaissance", "v"), civil, time.Date(1985, 7, 14, 23, 30, 0, 0, time.FixedZone("UTC-2", -2*3600)), "15/07/1985", ""},
		"ISO date text":             {mapping("birth", civil+"/dateNaissance", "v"), civil, "1985-07-14", "14/07/1985", ""},
		"RFC 3339 text":             {mapping("birth", civil+"/dateNaissance", "v"), civil, "1985-07-14T00:00:00Z", "14/07/1985", ""},
		"profile date text":         {mapping("birth", civil+"/dateNaissance", "v"), civil, "14/07/1985", "14/07/1985", ""},
		"unreadable date":           {mapping("birth", civil+"/dateNaissance", "v"), civil, "14 juillet 1985", "", "INVALID_SOURCE_DATE"},
		"impossible date":           {mapping("birth", civil+"/dateNaissance", "v"), civil, "31/02/1985", "", "INVALID_SOURCE_DATE"},
		"integer text":              {mapping("birth", civil+"/lieuNaissance", "v"), civil, int32(75056), "75056", ""},
		"float amount":              {balance, account, 100.01, "100.01", ""},
		"integer minor amount":      {cents, account, int32(10001), "100.01", ""},
		"not a number":              {balance, account, math.NaN(), "", "SOURCE_TYPE_UNSUPPORTED"},
	} {
		t.Run(name, func(t *testing.T) {
			m := tc.mapping
			m.ID = "birth"
			m.Source.Field = "v"
			block := models.TracfinBlock{ID: "b", CatalogID: tc.block, Selection: selection}
			draft := models.TracfinDraft{Document: models.TracfinDocument{Root: block}}
			got := ResolvePrefill(models.TracfinConfiguration{Mappings: []models.TracfinMapping{m}}, draft, SourceSet{Records: map[SourceKey]SourceRecord{{BlockID: "b", BindingID: "root"}: {Table: "User", RecordID: "U", Data: map[string]any{"v": tc.value}}}})
			if tc.code != "" {
				require.Empty(t, got.Proposals)
				require.Equal(t, tc.code, got.Diagnostics[0].Code)
				return
			}
			require.Empty(t, got.Diagnostics)
			require.Len(t, got.Proposals, 1)
			candidate := got.Proposals[0].Candidates[0]
			if candidate.Decimal != nil {
				require.Equal(t, models.TracfinDecimal(tc.want), *candidate.Decimal)
			} else {
				require.Equal(t, tc.want, *candidate.Text)
			}
		})
	}
}
