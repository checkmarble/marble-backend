package evaluate

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type Payload struct {
	Function ast.Function
	Payload  models.PayloadReader
}

func NewPayload(f ast.Function, payload models.PayloadReader) Payload {
	return Payload{
		Function: ast.FUNC_PAYLOAD,
		Payload:  payload,
	}
}

func (p Payload) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	payloadFieldName, err := adaptArgumentToString(arguments.Args[0])
	if err != nil {
		return nil, MakeAdaptedArgsErrors([]error{err})
	}

	value, err := p.Payload.ReadFieldFromPayload(models.FieldName(payloadFieldName))
	if err != nil {
		return MakeEvaluateError(errors.Wrap(models.ErrPayloadFieldNotFound,
			fmt.Sprintf("payload var does not exist: %s", payloadFieldName)))
	}

	if value == nil {
		return MakeEvaluateError(errors.Wrap(models.ErrNullFieldRead,
			fmt.Sprintf("value is null in payload field '%s'", payloadFieldName)))
	}

	return value, nil
}
