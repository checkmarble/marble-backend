package evaluate

import (
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models/ast"
)

type StringInList struct {
	Function ast.Function
}

func NewStringInList(f ast.Function) StringInList {
	return StringInList{
		Function: f,
	}
}

func (f StringInList) Evaluate(arguments ast.Arguments) (any, []error) {

	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}

	left, errLeft := adaptArgumentToString(leftAny)
	right, errRight := adaptArgumentToListOfStrings(rightAny)

	errs := MakeAdaptedArgsErrors([]error{errLeft, errRight})
	if len(errs) > 0 {
		return nil, errs
	}

	if f.Function == ast.FUNC_IS_IN_LIST {
		return stringInList(left, right), nil
	} else if f.Function == ast.FUNC_IS_NOT_IN_LIST {
		return !stringInList(left, right), nil
	} else {
		return MakeEvaluateError(fmt.Errorf("StringInList does not support %s function", f.Function.DebugString()))
	}
}

func stringInList(str string, list []string) bool {

	return slices.Contains(list, str)
}
