package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func TestUndefined(t *testing.T) {
	_, errs := Undefined{}.Evaluate(context.TODO(), ast.Arguments{})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrUndefinedFunction)
	}
}
