package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeNow(t *testing.T) {
	result, errs := TimeFunctions{ast.FUNC_TIME_NOW}.Evaluate(ast.Arguments{})
	assert.Empty(t, errs)
	assert.WithinDuration(t, time.Now(), result.(time.Time), 1*time.Millisecond)
}

func TestParseTime(t *testing.T) {
	result, errs := TimeFunctions{ast.FUNC_PARSE_TIME}.Evaluate(ast.Arguments{Args: []any{"2021-07-07T00:00:00Z"}})
	assert.Empty(t, errs)
	assert.Equal(t, time.Date(2021, 7, 7, 0, 0, 0, 0, time.UTC), result.(time.Time))
}

func TestParseTime_fail(t *testing.T) {
	_, errs := TimeFunctions{ast.FUNC_PARSE_TIME}.Evaluate(ast.Arguments{Args: []any{"2021-07-07 00:00:00Z"}})
	if assert.Len(t, errs, 1) {
		assert.Error(t, errs[0])
	}
}
