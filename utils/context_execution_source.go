package utils

import "context"

const (
	ExecutionSourceRiverJob  = "river_job"
	ExecutionSourceManualJob = "manual_job"
)

type ExecutionSource struct {
	Type    string
	JobID   int64
	JobKind string
}

func StoreExecutionSourceInContext(ctx context.Context, source ExecutionSource) context.Context {
	return context.WithValue(ctx, ContextKeyExecutionSource, source)
}

func ExecutionSourceFromContext(ctx context.Context) (ExecutionSource, bool) {
	source, found := ctx.Value(ContextKeyExecutionSource).(ExecutionSource)
	return source, found
}
