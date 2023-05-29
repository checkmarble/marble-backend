package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type ReadPayload struct {
	Payload map[string]any
}

func (f ReadPayload) Evaluate(arguments ast.Arguments) (any, error) {
	fieldName, err := arguments.StringNamedArgument(ast.FUNC_READ_PAYLOAD_ARGUMENT_FIELD_NAME)
	if err != nil {
		return nil, err
	}

	if value, ok := f.Payload[fieldName]; ok {
		return value, nil
	}

	return 0, fmt.Errorf("field '%s' not found in payload", fieldName)
}
