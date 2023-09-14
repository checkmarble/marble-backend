package evaluate_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseAccessValuesWrongArg(t *testing.T) {
	databaseAccessEval := evaluate.DatabaseAccess{}
	_, errs := databaseAccessEval.Evaluate(ast.Arguments{Args: []any{}})
	if assert.Len(t, errs, 2) {
		assert.ErrorIs(t, errs[0], ast.ErrMissingNamedArgument)
		assert.ErrorIs(t, errs[1], ast.ErrMissingNamedArgument)
	}
}

func TestDatabaseAccessValuesDryRun(t *testing.T) {
	databaseAccessEval := evaluate.DatabaseAccess{
		DataModel:       utils.GetDummyDataModel(),
		ReturnFakeValue: true,
	}
	var testDatabaseAccessNamedArgs = map[string]any{
		"tableName": string(utils.DummyTableNameFirst),
		"fieldName": string(utils.DummyFieldNameId),
		"path":      []any{},
	}

	value, errs := databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, fmt.Sprintf("fake value for DbAccess:%s..%s", testDatabaseAccessNamedArgs["tableName"], testDatabaseAccessNamedArgs["fieldName"]), value)

	testDatabaseAccessNamedArgs["fieldName"] = string(utils.DummyFieldNameForBool)
	testDatabaseAccessNamedArgs["path"] = []any{string(utils.DummyTableNameSecond)}
	value, errs = databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, true, value)

	testDatabaseAccessNamedArgs["fieldName"] = string(utils.DummyFieldNameForInt)
	value, errs = databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, 1, value)

	testDatabaseAccessNamedArgs["fieldName"] = string(utils.DummyFieldNameForFloat)
	value, errs = databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, 1.0, value)

	testDatabaseAccessNamedArgs["fieldName"] = string(utils.DummyFieldNameForTimestamp)
	timestamp, _ := time.Parse(time.RFC3339, time.RFC3339)
	value, errs = databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, timestamp, value)
}
