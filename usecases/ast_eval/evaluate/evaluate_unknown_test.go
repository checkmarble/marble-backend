package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnknown(t *testing.T) {
	_, err := Unknown{}.Evaluate(ast.Arguments{})
	assert.Error(t, err)
}
