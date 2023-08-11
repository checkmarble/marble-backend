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
