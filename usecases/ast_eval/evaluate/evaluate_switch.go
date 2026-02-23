package evaluate

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models/ast"
)

type Switch struct{}

func (p Switch) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	nodes, err := adaptArgumentToListOfThings[ast.ScoreComputationResult](arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}

	for _, node := range nodes {
		if node.Triggered {
			return node, nil
		}
	}

	return ast.ScoreComputationResult{}, []error{fmt.Errorf("no case triggered on switch")}
}
