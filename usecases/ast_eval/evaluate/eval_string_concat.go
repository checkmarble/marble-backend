package evaluate

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		switch v := arg.(type) {
		case nil:
			continue
		case string, int, float64:
			fmt.Fprintf(&sb, "%v", arg)

			if withSeparator && idx < len(arguments.Args)-1 {
				sb.WriteString(separator)
			}
		case time.Time:
			// If time.Time, get rid of the StringConcat wrapper that will not be used for screenings.
			return v, nil
		default:
			return nil, []error{errors.New("argument is not supported for StringConcat")}
		}
	}

	return sb.String(), nil
}
