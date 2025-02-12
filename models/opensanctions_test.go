package models

import (
	"testing"

	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/stretchr/testify/assert"
)

func TestOpenSanctionsAbstractTypeMapping(t *testing.T) {
	tts := []struct {
		kind         string
		outputLength int
		outputs      []string
	}{
		{"Vehicle", 2, []string{"Airplane", "Vessel"}},
	}

	for _, tt := range tts {
		req := SanctionCheckRefineRequest{Type: tt.kind, Query: OpenSanctionCheckFilter{
			"name": []string{"value"},
		}}

		queries := AdaptRefineRequestToMatchable(req)
		types := pure_utils.Map(queries, func(q OpenSanctionsCheckQuery) string {
			return q.Type
		})

		assert.Len(t, queries, tt.outputLength)

		for _, c := range tt.outputs {
			assert.Contains(t, types, c)
		}

		valuesEqual := true

		for _, q := range queries {
			assert.Len(t, q.Filters["name"], 1)
			if q.Filters["name"][0] != "value" {
				valuesEqual = false
			}
		}

		assert.True(t, valuesEqual)
	}
}
