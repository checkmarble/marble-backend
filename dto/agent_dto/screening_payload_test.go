package agent_dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realistic (trimmed) OFAC person payload with an attribute record (Sanction),
// a link, referents, and a relationship edge (Family) pointing to a relative.
const sampleScreeningPayload = `{
	"id": "NK-1",
	"caption": "Ayesha QADHAFI",
	"schema": "Person",
	"score": 0.9,
	"referents": ["ofac-12610", "gb-hmt-11635", "unsc-690757"],
	"properties": {
		"name": ["Ayesha QADHAFI"],
		"topics": ["sanction"],
		"sourceUrl": ["https://sanctionssearch.ofac.treas.gov/Details.aspx?id=12610"],
		"sanctions": [{
			"id": "ofac-sanction",
			"caption": "LIBYA2",
			"schema": "Sanction",
			"referents": [],
			"first_seen": "2023-04-20T10:27:20",
			"properties": {
				"program": ["LIBYA2"],
				"reason": ["Executive Order 13566 (Libya)"],
				"sourceUrl": ["https://www.treasury.gov/x"],
				"programUrl": ["https://ofac.treasury.gov/y"]
			}
		}],
		"familyPerson": [{
			"id": "fam-1",
			"caption": "Ayesha - Muammar",
			"schema": "Family",
			"properties": {
				"relationship": ["daughter"],
				"relative": [{
					"id": "NK-2",
					"caption": "Muammar QADHAFI",
					"schema": "Person",
					"referents": ["x", "y"],
					"properties": {
						"name": ["Muammar QADHAFI"],
						"birthDate": ["1942"],
						"sourceUrl": ["https://example.com/z"]
					}
				}]
			}
		}]
	}
}`

func TestSanitizeScreeningPayloadForLLM(t *testing.T) {
	out := parse(t, SanitizeScreeningPayloadForLLM([]byte(sampleScreeningPayload)))

	t.Run("drops top-level referents and link properties, keeps base fields", func(t *testing.T) {
		assert.NotContains(t, out, "referents")
		assert.Equal(t, "NK-1", out["id"])
		assert.EqualValues(t, 0.9, out["score"])

		props := out["properties"].(map[string]any)
		assert.Equal(t, []any{"Ayesha QADHAFI"}, props["name"])
		assert.Equal(t, []any{"sanction"}, props["topics"])
		assert.NotContains(t, props, "sourceUrl", "url-only property is dropped")
	})

	t.Run("keeps attribute records (Sanction) with their scalar detail, minus links and referents", func(t *testing.T) {
		props := out["properties"].(map[string]any)
		sanction := props["sanctions"].([]any)[0].(map[string]any)

		assert.Equal(t, "Sanction", sanction["schema"])
		assert.NotContains(t, sanction, "referents")
		assert.NotContains(t, sanction, "first_seen", "noisy timestamps dropped from nested records")

		sp := sanction["properties"].(map[string]any)
		assert.Equal(t, []any{"LIBYA2"}, sp["program"])
		assert.Equal(t, []any{"Executive Order 13566 (Libya)"}, sp["reason"])
		assert.NotContains(t, sp, "sourceUrl")
		assert.NotContains(t, sp, "programUrl")
	})

	t.Run("keeps the relationship edge but collapses the relative to base info + name", func(t *testing.T) {
		props := out["properties"].(map[string]any)
		fam := props["familyPerson"].([]any)[0].(map[string]any)

		assert.Equal(t, "Family", fam["schema"])
		famProps := fam["properties"].(map[string]any)
		assert.Equal(t, []any{"daughter"}, famProps["relationship"], "relationship type is kept")

		relative := famProps["relative"].([]any)[0].(map[string]any)
		assert.Equal(t, "Muammar QADHAFI", relative["caption"])
		assert.Equal(t, "NK-2", relative["id"])
		assert.NotContains(t, relative, "referents")

		relProps := relative["properties"].(map[string]any)
		assert.Equal(t, []any{"Muammar QADHAFI"}, relProps["name"], "name is kept")
		assert.NotContains(t, relProps, "birthDate", "relative's details are dropped")
		assert.NotContains(t, relProps, "sourceUrl")
	})

	t.Run("non-object payload returned unchanged", func(t *testing.T) {
		payload := []byte(`["not", "an", "object"]`)
		assert.JSONEq(t, string(payload), string(SanitizeScreeningPayloadForLLM(payload)))
	})

	t.Run("empty payload", func(t *testing.T) {
		assert.Empty(t, SanitizeScreeningPayloadForLLM(nil))
	})
}

func parse(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}
