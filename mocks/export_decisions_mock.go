package mocks

import (
	"io"

	"github.com/stretchr/testify/mock"
)

type ExportDecisionsMock struct {
	mock.Mock
}

func (e *ExportDecisionsMock) ExportDecisions(scheduledExecutionId string, dest io.Writer) (int, error) {
	args := e.Called(scheduledExecutionId, dest)
	return args.Int(0), args.Error(1)
}
