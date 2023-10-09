package evaluate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models/ast"
)

var timeArithmetic = TimeArithmetic{ast.FUNC_TIME_ADD}

func TestTimeArithmetic(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"timestampField": time.Date(2021, 7, 7, 0, 0, 0, 0, time.UTC),
			"duration":       "PT1H",
			"sign":           "+",
		},
	}
	expectedResult := time.Date(2021, 7, 7, 1, 0, 0, 0, time.UTC)

	result, errs := timeArithmetic.Evaluate(arguments)
	assert.Empty(t, errs)
	assert.Equal(t, expectedResult, result.(time.Time))
}

func TestTimeArithmetic_minus(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"timestampField": time.Date(2021, 7, 7, 1, 0, 0, 0, time.UTC),
			"duration":       "PT1H",
			"sign":           "-",
		},
	}
	expectedResult := time.Date(2021, 7, 7, 0, 0, 0, 0, time.UTC)

	result, errs := timeArithmetic.Evaluate(arguments)
	assert.Empty(t, errs)
	assert.Equal(t, expectedResult, result.(time.Time))
}

func TestTimeArithmetic_invalid_sign(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"timestampField": time.Date(2021, 7, 7, 0, 0, 0, 0, time.UTC),
			"duration":       "PT1H",
			"sign":           "invalid",
		},
	}

	_, errs := timeArithmetic.Evaluate(arguments)
	if assert.Len(t, errs, 1) {
		assert.Error(t, errs[0])
	}
}

func TestTimeArithmetic_invalid_timestampField(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"timestampField": 0,
			"duration":       "PT1H",
			"sign":           "+",
		},
	}

	_, errs := timeArithmetic.Evaluate(arguments)
	if assert.Len(t, errs, 1) {
		assert.Error(t, errs[0])
	}
}

func TestTimeArithmetic_invalid_duration(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"timestampField": time.Date(2021, 7, 7, 0, 0, 0, 0, time.UTC),
			"duration":       "invalid",
			"sign":           "+",
		},
	}

	_, errs := timeArithmetic.Evaluate(arguments)
	if assert.Len(t, errs, 1) {
		assert.Error(t, errs[0])
	}
}

func TestTimeArithmetic_missing_timestampField(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"duration": "PT1H",
			"sign":     "+",
		},
	}

	_, errs := timeArithmetic.Evaluate(arguments)
	if assert.Len(t, errs, 1) {
		assert.Error(t, errs[0])
	}
}

func TestTimeArithmetic_missing_duration(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"timestampField": time.Date(2021, 7, 7, 0, 0, 0, 0, time.UTC),
			"sign":           "+",
		},
	}

	_, errs := timeArithmetic.Evaluate(arguments)
	if assert.Len(t, errs, 1) {
		assert.Error(t, errs[0])
	}
}

func TestTimeArithmetic_missing_sign(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"timestampField": time.Date(2021, 7, 7, 0, 0, 0, 0, time.UTC),
			"duration":       "PT1H",
		},
	}

	_, errs := timeArithmetic.Evaluate(arguments)
	if assert.Len(t, errs, 1) {
		assert.Error(t, errs[0])
	}
}

var invalidTimeArithmetic = TimeArithmetic{}

func TestTimeArithmetic_invalid_function(t *testing.T) {
	arguments := ast.Arguments{
		NamedArgs: map[string]any{
			"timestampField": time.Date(2021, 7, 7, 0, 0, 0, 0, time.UTC),
			"duration":       "PT1H",
			"sign":           "+",
		},
	}

	_, errs := invalidTimeArithmetic.Evaluate(arguments)
	if assert.Len(t, errs, 1) {
		assert.Error(t, errs[0])
	}
}
