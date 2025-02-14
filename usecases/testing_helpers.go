package usecases

import (
	"context"
	"strings"

	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

func pipe[T any](fns ...func(t T) T) func(T) T {
	return func(t T) T {
		for _, fn := range fns {
			t = fn(t)
		}
		return t
	}
}

func escapeSql(str string) string {
	// replace all (,),$ by the escaped equivalent
	return pipe(
		func(s string) string { return strings.ReplaceAll(s, "(", "\\(") },
		func(s string) string { return strings.ReplaceAll(s, ")", "\\)") },
		func(s string) string { return strings.ReplaceAll(s, "$", "\\$") },
	)(str)
}

type anyUuid struct{}

func (a anyUuid) Match(v any) bool {
	str, ok := v.(string)
	if !ok {
		return false
	}
	_, err := uuid.Parse(str)
	return err == nil
}

func matchContext(ctx context.Context) bool     { return true }
func matchExec(exec repositories.Executor) bool { return true }
