package mocks

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type TransactionFactory struct {
	mock.Mock
	TxMock *Transaction
}

func (t *TransactionFactory) Transaction(databaseSchema models.DatabaseSchema, fn func(tx repositories.Transaction) error) error {
	args := t.Called(databaseSchema, fn)
	err := fn(t.TxMock)
	if err != nil {
		return err
	}
	return args.Error(0)
}
