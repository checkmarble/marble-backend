package engine

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func TestRelationError_CopiedBoolean(t *testing.T) {
	n := &core.Node{Type: core.TypeBoolean}
	cases := []struct {
		name, value, raw, just string
		ok                     bool
	}{
		{"oui to true", "true", "OUI", "", true},
		{"non to false", "false", "non", "", true},
		{"1 to true", "true", "1", "", true},
		{"0 to false", "false", "0", "", true},
		{"yes to false is wrong", "false", "Yes", "", false},
		{"false to true is wrong", "true", "false", "peu importe", false},
		{"non-literal needs justification", "true", "client à risque", "", false},
		{"non-literal with justification", "true", "client à risque", "le texte dit risque", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := relationError(n, c.value, c.raw, c.just)
			if c.ok {
				require.Empty(t, msg)
			} else {
				require.NotEmpty(t, msg)
			}
		})
	}
}

func TestRelationError_NumbersFromNonNumericRaw(t *testing.T) {
	for _, typ := range []core.FieldType{core.TypeInteger, core.TypeDecimal} {
		n := &core.Node{Type: typ}
		cases := []struct {
			name, value, raw string
			ok               bool
		}{
			{"token of an address", "12", "12 rue du Test", true},
			{"token inside text", "75", "Paris 75009", false},
			{"token among several", "9", "12 rue du Test, 9 bis", true},
			{"thousands space", "1000", "1 000", true},
			{"thousands nbsp", "1000", "1 000", true},
			{"thousands narrow nbsp decimal", "1000.5", "1 000,50", true},
			{"plain equal", "10.5", "10,50", true},
			{"other number", "13", "12 rue du Test", false},
			{"no number", "12", "rue du Test", false},
		}
		for _, c := range cases {
			t.Run(string(typ)+"/"+c.name, func(t *testing.T) {
				msg := relationError(n, c.value, c.raw, "")
				if c.ok {
					require.Empty(t, msg)
				} else {
					require.NotEmpty(t, msg)
				}
			})
		}
	}
}

func TestParisDay_NaiveTimestampIsParisTime(t *testing.T) {
	// 23:30 without zone is 23:30 in Paris, not 23:30 UTC (which would be the
	// next Paris day).
	d, ok := parisDay("2026-03-01T23:30:00")
	require.True(t, ok)
	require.Equal(t, "2026-03-01", d)
	d, ok = parisDay("2026-03-01 23:30:00")
	require.True(t, ok)
	require.Equal(t, "2026-03-01", d)
	// With a zone: converted.
	d, ok = parisDay("2026-03-01T23:30:00Z")
	require.True(t, ok)
	require.Equal(t, "2026-03-02", d)
	day, ok := calendarDay("2026-03-01T23:30:00")
	require.True(t, ok)
	require.Equal(t, "2026-03-01", day)
}
