package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/repositories"
)

type TransactionFactory struct {
	mock.Mock
	ExecMock *Executor
}

func (t *TransactionFactory) Transaction(ctx context.Context, fn func(exec repositories.Executor) error) error {
	args := t.Called(ctx, fn)
	err := fn(t.ExecMock)
	if err != nil {
		return err
	}
	return args.Error(0)
}

func (t *TransactionFactory) TransactionInOrgSchema(ctx context.Context, organizationId string, f func(tx repositories.Executor) error) error {
	args := t.Called(ctx, organizationId, f)
	err := f(t.ExecMock)
	if err != nil {
		return err
	}
	return args.Error(0)
}

type ExecutorFactory struct {
	mock.Mock
}

func (e *ExecutorFactory) NewClientDbExecutor(ctx context.Context, organizationId string) (repositories.Executor, error) {
	args := e.Called(ctx, organizationId)
	return args.Get(0).(repositories.Executor), args.Error(1)
}

func (e *ExecutorFactory) NewExecutor() repositories.Executor {
	args := e.Called()
	return args.Get(0).(repositories.Executor)
}
