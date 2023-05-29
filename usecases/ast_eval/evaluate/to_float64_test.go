package evaluate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToFloat64(t *testing.T) {
	expected := float64(13)

	check := func(v any) {
		result, err := ToFloat64(v)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	}

	check(int8(13))
	check(int16(13))
	check(int32(13))
	check(int64(13))

	check(int(13))
	check(uint(13))

	check(uint8(13))
	check(uint16(13))
	check(uint32(13))
	check(uint64(13))

	check(float32(13))
	check(float64(13))
}
