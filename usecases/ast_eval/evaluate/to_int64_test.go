package evaluate

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToInt64(t *testing.T) {

	expected := int64(13)

	check := func(v any) {
		result, err := ToInt64(v)
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
}

func TestInvalidNumbers(t *testing.T) {

	checkErr := func(v interface{}) {
		_, err := ToInt64(v)
		assert.Error(t, err)
	}

	// to big
	checkErr(uint64(math.MaxUint64))

	// checkErr
	checkErr(float32(0))
	checkErr(float64(0))
	checkErr("0")
}
