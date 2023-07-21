package ast

type NodeEvaluation struct {
	ReturnValue     any
	EvaluationError error

	Children      []NodeEvaluation
	NamedChildren map[string]NodeEvaluation
}

func (root NodeEvaluation) AllErrors() (errs []error) {

	var addEvaluationErrors func(NodeEvaluation)

	addEvaluationErrors = func(child NodeEvaluation) {
		if child.EvaluationError != nil {
			errs = append(errs, child.EvaluationError)
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
