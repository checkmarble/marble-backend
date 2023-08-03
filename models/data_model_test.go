package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataType(t *testing.T) {

	// DataType is serialized in database
	// So we want to make sure the values stay stable
	assert.Equal(t, int(UnknownDataType), -1)
	assert.Equal(t, int(Bool), 0)
	assert.Equal(t, int(Int), 1)
	assert.Equal(t, int(Float), 2)
	assert.Equal(t, int(String), 3)
	assert.Equal(t, int(Timestamp), 4)
}
