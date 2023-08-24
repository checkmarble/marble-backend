package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnknown(t *testing.T) {
	_, errs := Unknown{}.Evaluate(ast.Arguments{})
	assert.Len(t, errs, 1)
}
