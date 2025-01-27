package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeWeights(t *testing.T) {
	tts := []struct {
		n Node
		c int
	}{
		{Node{Function: FUNC_AND, Children: []Node{{Function: FUNC_DB_ACCESS}, {Function: FUNC_PAYLOAD}}}, 60},
		{Node{Function: FUNC_AND, Children: []Node{{Function: FUNC_DB_ACCESS}, {
			Function: FUNC_ADD, Children: []Node{{
				Function: FUNC_AGGREGATOR,
				Children: []Node{{Function: FUNC_CUSTOM_LIST_ACCESS}, {Function: FUNC_PAYLOAD}},
			}},
		}}}, 140},
	}

	for _, tt := range tts {
		assert.Equal(t, tt.c, tt.n.Cost())
	}
}
