package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
)

type Payload struct {
	Function ast.Function
	Payload models.PayloadReader
}

func NewPayload(f ast.Function, payload models.PayloadReader) Payload {
	return Payload{
		Function: ast.FUNC_PAYLOAD,
		Payload: payload,
	}
}

func (p Payload) Evaluate(arguments ast.Arguments) (any, error) {
	payloadFieldName, err := adaptArgumentToString(p.Function, arguments.Args[0])
	if err != nil {
		return nil, fmt.Errorf("payload field name is not a string %w", models.ErrRuntimeExpression)
	}
	value, err := p.Payload.ReadFieldFromPayload(models.FieldName(payloadFieldName));
	if err != nil {
		return nil, fmt.Errorf("payload var does not exist: %s", payloadFieldName)
	}

	if value == nil {
		return nil, fmt.Errorf("value is null in payload field %s, %w", payloadFieldName, models.NullFieldReadError)
	}

	return value, nil
}
