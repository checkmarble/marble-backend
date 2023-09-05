package evaluate

import (
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

var list = List{}

func TestList(t *testing.T) {
	arguments := ast.Arguments{
		Args: []any{1, 2, 3},
	}
	expectedResult := []int{1, 2, 3}
	result, errs := list.Evaluate(arguments)
	assert.Empty(t, errs)
	assert.ObjectsAreEqualValues(expectedResult, result)
}
