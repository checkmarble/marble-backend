package transaction

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type TransactionFactory interface {
	Transaction(databaseSchema models.DatabaseSchema, fn func(tx repositories.Transaction) error) error
}
