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
		output      models.OpenSanctionCheckFilter
	}{
		{
			`{"person":{"name":"bob","id_number":"123"}}`,
			"Person",
			models.OpenSanctionCheckFilter{"name": []string{"bob"}, "idNumber": []string{"123"}},
		},
		{
			`{"organization":{"name":"acme","registration_number":"123","country":"uk"}}`,
			"Organization",
			models.OpenSanctionCheckFilter{
				"name":               []string{"acme"},
				"registrationNumber": []string{"123"}, "country": []string{"uk"},
			},
		},
		{
			`{"vehicle":{"registration_number":"987"}}`,
			"Vehicle",
			models.OpenSanctionCheckFilter{"registrationNumber": []string{"987"}},
		},
		{
			`{"thing":{"name":"bob"}}`,
			"Thing",
			models.OpenSanctionCheckFilter{"name": []string{"bob"}},
		},
	}

	for _, tt := range tts {
		refineDto := SanctionCheckRefineDto{}

		assert.NoError(t, json.Unmarshal([]byte(tt.query), &refineDto.Query))

		parsed := AdaptSanctionCheckRefineDto(refineDto)

		assert.Equal(t, tt.kind, parsed.Type)
		assert.Equal(t, tt.output, parsed.Query)
	}
}
