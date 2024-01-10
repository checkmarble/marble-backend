package evaluate

import (
	"fmt"
	"math"

	"github.com/cockroachdb/errors"
)

func ToInt64(v any) (int64, error) {
	switch v := v.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil

	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		if v > math.MaxInt64 {
			return 0, errors.New(fmt.Sprintf("uint64 value %d is too large to be converted to int64", v))
		}
		return int64(v), nil
	default:
		return 0, errors.New(fmt.Sprintf("value '%v' cannot be converted to int64", v))
	}
}
