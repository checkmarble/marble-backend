package utils

import (
	"context"
	"testing"
)

func TestExecutionSourceContext(t *testing.T) {
	source := ExecutionSource{
		Type:    ExecutionSourceRiverJob,
		JobID:   123,
		JobKind: "async_decision",
	}

	got, found := ExecutionSourceFromContext(StoreExecutionSourceInContext(context.Background(), source))
	if !found {
		t.Fatal("execution source not found")
	}
	if got != source {
		t.Fatalf("execution source = %+v, want %+v", got, source)
	}
}
