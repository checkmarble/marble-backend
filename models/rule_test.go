package models

import (
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
)

func TestRule_HasSameFormula(t *testing.T) {
	nodeA := &ast.Node{Constant: "a"}
	nodeAAgain := &ast.Node{Constant: "a"}
	nodeB := &ast.Node{Constant: "b"}

	t.Run("structurally identical ASTs are same formula", func(t *testing.T) {
		r1 := Rule{FormulaAstExpression: nodeA}
		r2 := Rule{FormulaAstExpression: nodeAAgain}
		assert.True(t, r1.HasSameFormula(r2))
	})

	t.Run("structurally different ASTs are different formula", func(t *testing.T) {
		r1 := Rule{FormulaAstExpression: nodeA}
		r2 := Rule{FormulaAstExpression: nodeB}
		assert.False(t, r1.HasSameFormula(r2))
	})
}
