package evaluate

import "marble/marble-backend/models/ast"

type DatabaseAccess struct {
}

func (f DatabaseAccess) Evaluate(arguments ast.Arguments) (any, error) {
	return 0, nil
}
