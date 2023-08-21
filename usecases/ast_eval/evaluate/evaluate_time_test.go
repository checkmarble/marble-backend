package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeNow(t *testing.T) {
	result, err := TimeFunctions{ast.FUNC_TIME_NOW}.Evaluate(ast.Arguments{})
	assert.NoError(t, err)
	assert.WithinDuration(t, time.Now(), result.(time.Time), 1*time.Millisecond)
}

func TestParseTime(t *testing.T) {
	result, err := TimeFunctions{ast.FUNC_PARSE_TIME}.Evaluate(ast.Arguments{Args: []any{"2021-07-07T00:00:00Z"}})
	assert.NoError(t, err)
	assert.Equal(t, time.Date(2021, 7, 7, 0, 0, 0, 0, time.UTC), result.(time.Time))

	_, err = TimeFunctions{ast.FUNC_PARSE_TIME}.Evaluate(ast.Arguments{Args: []any{"2021-07-07 00:00:00Z"}})
	assert.Error(t, err)
}
