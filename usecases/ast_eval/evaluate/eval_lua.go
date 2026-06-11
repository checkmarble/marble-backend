package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type Lua struct {
	ClientObject models.ClientObject
	PivotObject  models.DataModelObject
}

func NewLua(payload models.ClientObject, pivot models.DataModelObject) Lua {
	return Lua{
		ClientObject: payload,
		PivotObject:  pivot,
	}
}

func (f Lua) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	code, err := AdaptNamedArgument(arguments.NamedArgs, "code", adaptArgumentToString)
	if err != nil {
		return nil, MakeAdaptedArgsErrors([]error{err})
	}

	sandbox := NewLuaSandbox()
	sandbox.SetClientObject(f.ClientObject.Data)
	sandbox.SetPivotObject(f.PivotObject.Data)

	v, err := sandbox.Run(code)

	errs := MakeAdaptedArgsErrors([]error{err})

	if len(errs) > 0 {
		return nil, errs
	}

	return v, nil
}
