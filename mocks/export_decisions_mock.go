package mocks

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"
)

type ExportDecisionsMock struct {
	mock.Mock
}

func (e *ExportDecisionsMock) ExportDecisions(ctx context.Context, organizationId string, scheduledExecutionId string, dest io.Writer) (int, error) {
	args := e.Called(ctx, organizationId, scheduledExecutionId, dest)
	return args.Int(0), args.Error(1)
}
