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
	payloadArg, ok := arguments.Args[0].(string)
	if !ok {
		return nil, fmt.Errorf("tableName is not a string %w", ErrRuntimeExpression)
	}
	payloadFieldName, err := adaptArgumentToString(p.Function, payloadArg)
	if err != nil {
		return nil, err
	}
	if value, err := p.Payload.ReadFieldFromPayload(models.FieldName(payloadFieldName)); err != nil {
		return value, nil
	}
	return 0, fmt.Errorf("payload var does not exist: %s", payloadArg)
}
