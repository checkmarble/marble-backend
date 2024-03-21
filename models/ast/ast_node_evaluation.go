package ast

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

type NodeEvaluation struct {
	ReturnValue any
	Errors      []error

	Children      []NodeEvaluation
	NamedChildren map[string]NodeEvaluation
}

func (root NodeEvaluation) AllErrors() (errs []error) {
	var addEvaluationErrors func(NodeEvaluation)

	addEvaluationErrors = func(child NodeEvaluation) {
		if child.Errors != nil {
			errs = append(errs, child.Errors...)
		}

		for _, child := range child.Children {
			addEvaluationErrors(child)
		}

		for _, child := range child.NamedChildren {
			addEvaluationErrors(child)
		}
	}

	addEvaluationErrors(root)
	return errs
}

type RootNodeEvaluation struct {
	ReturnValue bool
	Errors      []error

	Children      []NodeEvaluation
	NamedChildren map[string]NodeEvaluation
}

func AdaptRootNodeEvaluation(root NodeEvaluation) (RootNodeEvaluation, error) {
	if returnValue, ok := root.ReturnValue.(bool); ok {
		return RootNodeEvaluation{
			ReturnValue:   returnValue,
			Errors:        root.Errors,
			Children:      root.Children,
			NamedChildren: root.NamedChildren,
		}, nil
	}

	return RootNodeEvaluation{}, errors.New(
		fmt.Sprintf("root ast expression does not return a boolean, '%v' instead", root.ReturnValue))
}
