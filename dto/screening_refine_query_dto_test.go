package dto

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestBuildRefineQuery(t *testing.T) {
	tts := []struct {
		query, kind string
		output      models.OpenSanctionsFilter
	}{
		{
			`{"Person":{"name":"bob","idNumber":"123"}}`,
			"Person",
			models.OpenSanctionsFilter{"name": []string{"bob"}, "idNumber": []string{"123"}},
		},
		{
			`{"organization":{"name":"acme","registrationNumber":"123","country":"uk"}}`,
			"Organization",
			models.OpenSanctionsFilter{
				"name":               []string{"acme"},
				"registrationNumber": []string{"123"}, "country": []string{"uk"},
			},
		},
		{
			`{"Vehicle":{"registrationNumber":"987"}}`,
			"Vehicle",
			models.OpenSanctionsFilter{"registrationNumber": []string{"987"}},
		},
		{
			`{"Thing":{"name":"bob"}}`,
			"Thing",
			models.OpenSanctionsFilter{"name": []string{"bob"}},
		},
	}

	for _, tt := range tts {
		refineDto := ScreeningRefineDto{}

		assert.NoError(t, json.Unmarshal([]byte(tt.query), &refineDto.Query))

		parsed := AdaptScreeningRefineDto(refineDto)

		assert.Equal(t, tt.kind, parsed.Type)
		assert.Equal(t, tt.output, parsed.Query)
	}
}
