package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUndefined(t *testing.T) {
	_, errs := Undefined{}.Evaluate(ast.Arguments{})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrUndefinedFunction)
	}
}
