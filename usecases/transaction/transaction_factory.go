package transaction

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type TransactionFactory interface {
	Transaction(ctx context.Context, databaseSchema models.DatabaseSchema, fn func(tx repositories.Transaction) error) error
}
