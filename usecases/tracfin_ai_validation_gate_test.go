package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

func TestAIValidationGateUnavailableOfficialXSD(t *testing.T) {
	makeValidation := func() *models.TracfinValidation {
		return &models.TracfinValidation{Layers: map[string]models.TracfinValidationLayer{
			"internal_structure": {Status: "passed"}, "references": {Status: "passed"}, "business": {Status: "passed"}, "xml_syntax": {Status: "passed"}, "official_xsd": {Status: "unavailable", Reason: "incompatible engine"},
		}}
	}
	t.Run("unavailable with complete internal checks", func(t *testing.T) {
		v := makeValidation()
		require.True(t, aiValidationPassed(v))
		require.Equal(t, "unavailable", v.Layers["official_xsd"].Status)
		require.Equal(t, "incompatible engine", v.Layers["official_xsd"].Reason)
	})
	for _, status := range []string{"failed", "incomplete", "", "unknown"} {
		t.Run("xsd "+status, func(t *testing.T) {
			v := makeValidation()
			v.Layers["official_xsd"] = models.TracfinValidationLayer{Status: status}
			require.False(t, aiValidationPassed(v))
		})
	}
	t.Run("passed XSD", func(t *testing.T) {
		v := makeValidation()
		v.Layers["official_xsd"] = models.TracfinValidationLayer{Status: "passed"}
		require.True(t, aiValidationPassed(v))
	})
	for _, layer := range []string{"internal_structure", "references", "business", "xml_syntax", "official_xsd"} {
		t.Run("missing "+layer, func(t *testing.T) {
			v := makeValidation()
			delete(v.Layers, layer)
			require.False(t, aiValidationPassed(v))
		})
	}
	t.Run("incomplete coverage", func(t *testing.T) {
		v := makeValidation()
		v.Layers["business"] = models.TracfinValidationLayer{Status: "incomplete"}
		require.False(t, aiValidationPassed(v))
	})
	t.Run("other unavailable layer", func(t *testing.T) {
		v := makeValidation()
		v.Layers["other"] = models.TracfinValidationLayer{Status: "unavailable"}
		require.False(t, aiValidationPassed(v))
	})
	t.Run("error diagnostic", func(t *testing.T) {
		v := makeValidation()
		v.Diagnostics = []models.TracfinDiagnostic{{Severity: "error"}}
		require.False(t, aiValidationPassed(v))
	})
	require.False(t, aiValidationPassed(nil))
}
