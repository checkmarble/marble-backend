package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

// Regression tests for the global review of the TRACFIN branch.

func TestTracfinConfigurationRejectsInvalidMappings(t *testing.T) {
	sex := func(codes ...models.TracfinCodeMapping) models.TracfinMapping {
		m := task8Mapping("sex", t8Civil+"/sexe", "Person", "User", "gender")
		m.Codes = codes
		return m
	}
	amount := task8Snapshot("amount", t8Op+"/montantTotal", "amount")
	amount.Unit = "minor"
	for name, tc := range map[string]struct {
		mapping models.TracfinMapping
		code    string
	}{
		"unknown target code":       {sex(models.TracfinCodeMapping{Source: "F", Target: "SEX9"}), "UNKNOWN_CATALOG_CODE"},
		"contradictory client code": {sex(models.TracfinCodeMapping{Source: "F", Target: "SEX1"}, models.TracfinCodeMapping{Source: "F", Target: "SEX2"}), "CONTRADICTORY_CODE_MAPPING"},
		"amount without currency":   {amount, "MISSING_TARGET_CURRENCY"},
		"partner root":              {task8Mapping("partner", t8Civil+"/nomNaissance", "Partner", "User", "name"), "INVALID_ENTITY_TYPE"},
	} {
		t.Run(name, func(t *testing.T) {
			w := task8NewWorld(models.DataModel{}, nil, nil, nil)
			_, err := w.uc.SaveConfiguration(w.ctx, w.org, models.TracfinConfigurationInput{Mappings: []models.TracfinMapping{tc.mapping}})
			require.ErrorIs(t, err, models.BadParameterError)
			require.ErrorContains(t, err, tc.code)
			require.ErrorIs(t, w.repo.configErr, models.NotFoundError, "an invalid configuration is not saved")
		})
	}
	w := task8NewWorld(models.DataModel{}, nil, nil, nil)
	saved, err := w.uc.SaveConfiguration(w.ctx, w.org, models.TracfinConfigurationInput{Mappings: []models.TracfinMapping{sex(models.TracfinCodeMapping{Source: "F", Target: "SEX2"})}})
	require.NoError(t, err)
	require.Equal(t, int64(1), saved.Revision)
}
