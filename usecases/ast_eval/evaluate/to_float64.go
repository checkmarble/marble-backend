package evaluate

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

func ToFloat64(v any) (float64, error) {
	switch v := v.(type) {

	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil

	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil

	case float32:
		return float64(v), nil
	case float64:
		return v, nil

	default:
		return 0, errors.New(fmt.Sprintf("value %v cannot be converted to float64", v))
	}
}
