package evaluate

import (
	"errors"
	"fmt"
	"marble/marble-backend/models/ast"

	"golang.org/x/exp/slices"
)

type StringInList struct {
	Function ast.Function
}

func NewStringInList(f ast.Function) StringInList {
	return StringInList{
		Function: f,
	}
}

func (f StringInList) Evaluate(arguments ast.Arguments) (any, error) {

	leftAny, rightAny, err := leftAndRight(f.Function, arguments.Args)
	if err != nil {
		return nil, err
	}

	left, errLeft := adaptArgumentToString(f.Function, leftAny)

	right, errRight := adaptArgumentToListOfStrings(f.Function, rightAny)

	if err := errors.Join(errLeft, errRight); err != nil {
		return nil, err
	}

	if f.Function == ast.FUNC_IS_IN_LIST {
		return stringInList(left, right), nil
	} else if f.Function == ast.FUNC_IS_NOT_IN_LIST {
		return !stringInList(left, right), nil
	} else {
		return false, fmt.Errorf("StringInList does not support %s function", f.Function.DebugString())
	}
}

func stringInList(str string, list []string) bool {

	return slices.Contains(list, str)
}
