package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/repositories"
)

type TransactionFactory struct {
	mock.Mock
	TxMock *Transaction
}

func (t *TransactionFactory) Transaction(ctx context.Context, fn func(exec repositories.Executor) error) error {
	args := t.Called(ctx, fn)
	err := fn(t.TxMock)
	if err != nil {
		return err
	}
	return args.Error(0)
}

func (t *TransactionFactory) TransactionInOrgSchema(ctx context.Context, organizationId string, f func(tx repositories.Executor) error) error {
	args := t.Called(ctx, organizationId, f)
	err := f(t.TxMock)
	if err != nil {
		return err
	}
	return args.Error(0)
}
