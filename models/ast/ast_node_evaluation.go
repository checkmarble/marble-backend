package ast

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

type NodeEvaluation struct {
	Function    Function
	ReturnValue any
	Errors      []error

	Children      []NodeEvaluation
	NamedChildren map[string]NodeEvaluation
}

func (root NodeEvaluation) FlattenErrors() []error {
	errs := make([]error, 0)

	errs = append(errs, root.Errors...)

	for _, child := range root.Children {
		errs = append(errs, child.FlattenErrors()...)
	}

	for _, child := range root.NamedChildren {
		errs = append(errs, child.FlattenErrors()...)
	}

	return errs
}

func (root NodeEvaluation) GetBoolReturnValue() (bool, error) {
	if root.ReturnValue == nil {
		return false, ErrNullFieldRead
	}

	if returnValue, ok := root.ReturnValue.(bool); ok {
		return returnValue, nil
	}

	return false, errors.New(
		fmt.Sprintf("root ast expression does not return a boolean, '%v' instead", root.ReturnValue))
}
