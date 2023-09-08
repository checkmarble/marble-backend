package evaluate_test

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval/evaluate"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestDatabaseAccessValuesWrongArg(t *testing.T) {
	databaseAccessEval := evaluate.DatabaseAccess{}
	_, errs := databaseAccessEval.Evaluate(ast.Arguments{Args: []any{}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrMissingNamedArgument)
	}
}

func TestDatabaseAccessValuesDryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	databaseAccessEval := evaluate.DatabaseAccess{
		DataModel:       getTestFirstDataModel(),
		ReturnFakeValue: true,
	}
	var testDatabaseAccessNamedArgs = map[string]any{
		"tableName": string(testTableNameFirst),
		"fieldName": string(testFieldNameId),
		"path":      []any{},
	}

	value, errs := databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, fmt.Sprintf("fake value for DbAccess:%s..%s", testDatabaseAccessNamedArgs["tableName"], testDatabaseAccessNamedArgs["fieldName"]), value)

	testDatabaseAccessNamedArgs["fieldName"] = string(testFieldNameForBool)
	testDatabaseAccessNamedArgs["path"] = []any{string(testTableNameSecond)}
	value, errs = databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, true, value)

	testDatabaseAccessNamedArgs["fieldName"] = string(testFieldNameForInt)
	value, errs = databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, 1, value)

	testDatabaseAccessNamedArgs["fieldName"] = string(testFieldNameForFloat)
	value, errs = databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, 1.0, value)

	testDatabaseAccessNamedArgs["fieldName"] = string(testFieldNameForTimestamp)
	timestamp, _ := time.Parse(time.RFC3339, time.RFC3339)
	value, errs = databaseAccessEval.Evaluate(ast.Arguments{NamedArgs: testDatabaseAccessNamedArgs})
	assert.Len(t, errs, 0)
	assert.Equal(t, timestamp, value)
}
