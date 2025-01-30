package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/stretchr/testify/assert"
)

func TestStringConcat(t *testing.T) {
	tts := []struct {
		in            []any
		out           string
		withSeparator bool
		separator     *string
		error         bool
	}{
		{[]any{"abc", "def"}, "abcdef", false, nil, false},
		{[]any{42, "abc", 12}, "42abc12", false, nil, false},
		{[]any{42, "abc", 12}, "42 abc 12", true, nil, false},
		{[]any{42, "abc", 12}, "42-abc-12", true, utils.Ptr("-"), false},
		{[]any{42, "abc", true}, "42 abc 12", true, nil, true},
		{[]any{"hello", nil, "world"}, "hello world", true, nil, false},
	}

	eval := StringConcat{}

	for _, tt := range tts {
		namedArgs := map[string]any{}

		if tt.withSeparator {
			namedArgs["with_separator"] = true
		}
		if tt.separator != nil {
			namedArgs["separator"] = *tt.separator
		}

		result, err := eval.Evaluate(context.TODO(), ast.Arguments{
			Args: tt.in, NamedArgs: namedArgs,
		})

		switch tt.error {
		case true:
			assert.NotEmpty(t, err)
		default:
			assert.Empty(t, err)
			assert.Equal(t, tt.out, result)
		}
	}
}
