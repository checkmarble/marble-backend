package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFunction(t *testing.T) {
	// The stability of int values of function are not critical, they are never serialized,
	// but it is nice to have them in order
	assert.Equal(t, int(FUNC_UNKNOWN), -1)
	assert.Equal(t, int(FUNC_CONSTANT), 0)
	assert.Equal(t, int(FUNC_ADD), 1)
}
