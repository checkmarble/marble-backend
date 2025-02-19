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
		name          string
		in            []any
		out           string
		withSeparator bool
		separator     *string
		error         bool
	}{
		{"nominal", []any{"abc", "def"}, "abcdef", false, nil, false},
		{"mixed types", []any{42, "abc", 12}, "42abc12", false, nil, false},
		{"mixed types and separator", []any{42, "abc", 12}, "42 abc 12", true, nil, false},
		{"mixed types custom separator", []any{42, "abc", 12}, "42-abc-12", true, utils.Ptr("-"), false},
		{"boolean input returns error", []any{42, "abc", true}, "", true, nil, true},
		{"with nil", []any{"hello", nil, "world"}, "hello world", true, nil, false},
		{"only nil", []any{nil, nil, nil}, "", true, nil, false},
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

		asserts := assert.New(t)
		switch tt.error {
		case true:
			asserts.NotEmpty(err, tt.name, "expected ast eval errors")
		default:
			asserts.Empty(err, tt.name, "unexpected ast eval errors")
			asserts.Equal(tt.out, result, tt.name, "incorrect result")
		}
	}
}
