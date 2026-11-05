package dto

import (
	"net/http"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaseFilters_BindsTypesFromQuery(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/cases?type[]=decision&type[]=continuous_screening", nil)
	require.NoError(t, err)

	var filters CaseFilters
	require.NoError(t, binding.Form.Bind(req, &filters))

	assert.Equal(t, []string{"decision", "continuous_screening"}, filters.Types)
}

func TestCaseFiltersParse_Types(t *testing.T) {
	for name, tc := range map[string]struct {
		input    []string
		expected []models.CaseType
	}{
		"absent":   {},
		"one":      {input: []string{"continuous_screening"}, expected: []models.CaseType{models.CaseTypeContinuousScreening}},
		"multiple": {input: []string{"decision", "continuous_screening"}, expected: []models.CaseType{models.CaseTypeDecision, models.CaseTypeContinuousScreening}},
	} {
		t.Run(name, func(t *testing.T) {
			filters, err := CaseFilters{Types: tc.input}.Parse()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, filters.Types)
		})
	}

	// "unknown" is the string form of CaseTypeUnknown: it must be rejected, not treated as "no filter".
	for _, input := range [][]string{{"unknown"}, {"DECISION"}, {"decision", "not_a_type"}, {""}} {
		t.Run("invalid", func(t *testing.T) {
			_, err := CaseFilters{Types: input}.Parse()
			require.Error(t, err)
			assert.True(t, errors.Is(err, models.BadParameterError))
		})
	}
}
