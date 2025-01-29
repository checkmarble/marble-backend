package evaluate

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
)

type StringConcat struct{}

func (f StringConcat) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	sb := strings.Builder{}
	withSeparator := false
	separator := " "

	if withSeparatorAny, ok := arguments.NamedArgs["with_separator"]; ok {
		if withSeparatorBool, ok := withSeparatorAny.(bool); ok {
			withSeparator = withSeparatorBool
		}
	}
	if separatorAny, ok := arguments.NamedArgs["separator"]; ok {
		if separatorStr, ok := separatorAny.(string); ok {
			separator = separatorStr
		}
	}

	for idx, arg := range arguments.Args {
		switch arg.(type) {
		case string, int, float64:
		default:
			return nil, []error{errors.New("argument is not supported for StringConcat")}
		}

		sb.WriteString(fmt.Sprintf("%v", arg))

		if withSeparator && idx < len(arguments.Args)-1 {
			sb.WriteString(separator)
		}
	}

	return sb.String(), nil
}
