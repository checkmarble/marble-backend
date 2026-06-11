package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type Payload struct {
	Function     ast.Function
	Full         bool
	ClientObject models.ClientObject
}

func NewPayload(f ast.Function, payload models.ClientObject) Payload {
	return Payload{
		Function:     ast.FUNC_PAYLOAD,
		ClientObject: payload,
	}
}

func NewFullPayload(f ast.Function, payload models.ClientObject) Payload {
	return Payload{
		Function:     ast.FUNC_PAYLOAD,
		Full:         true,
		ClientObject: payload,
	}
}

func (p Payload) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	if p.Full {
		return p.ClientObject, nil
	}

	payloadFieldName, err := adaptArgumentToString(arguments.Args[0])
	if err != nil {
		return nil, MakeAdaptedArgsErrors([]error{err})
	}

	value := p.ClientObject.Data[payloadFieldName]

	valueStr, ok := value.(string)
	if ok {
		return pure_utils.Normalize(valueStr), nil
	}

	return value, nil
}
