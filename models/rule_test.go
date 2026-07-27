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

	t.Run("both nil is same formula", func(t *testing.T) {
		r1 := Rule{FormulaAstExpression: nil}
		r2 := Rule{FormulaAstExpression: nil}
		assert.True(t, r1.HasSameFormula(r2))
	})

	t.Run("one nil one not is different formula", func(t *testing.T) {
		r1 := Rule{FormulaAstExpression: nil}
		r2 := Rule{FormulaAstExpression: nodeA}
		assert.False(t, r1.HasSameFormula(r2))
		assert.False(t, r2.HasSameFormula(r1))
	})

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
