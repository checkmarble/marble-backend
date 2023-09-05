package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type Transaction struct {
	mock.Mock
}

func (t *Transaction) DatabaseSchema() models.DatabaseSchema {
	args := t.Called()
	return args.Get(0).(models.DatabaseSchema)
}
