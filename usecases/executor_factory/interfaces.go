package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type TransactionFactory interface {
	TransactionInOrgSchema(ctx context.Context, organizationId uuid.UUID,
		f func(tx repositories.Transaction) error) error
	Transaction(ctx context.Context, fn func(tx repositories.Transaction) error) error
}

// ExecutorFactory provides executors for the Marble database and an
// organization's client database. A pinned executor holds a single database
// connection across operations, which is needed for session-scoped features
// such as advisory locks. Pinning a connection does not start a transaction.
type ExecutorFactory interface {
	// NewExecutor returns an executor for the Marble database. Depending on the
	// implementation, it may reuse an existing connection.
	NewExecutor() repositories.Executor
	// NewClientDbExecutor returns an executor for the organization's client database.
	NewClientDbExecutor(ctx context.Context, organizationId uuid.UUID) (repositories.Executor, error)
	// NewPinnedExecutor returns a Marble database executor pinned to one
	// connection. The caller must invoke the release function after use.
	NewPinnedExecutor(ctx context.Context) (repositories.Executor, func(), error)
	// NewPinnedClientDbExecutor returns an executor pinned to one connection in
	// the organization's client database. The caller must invoke the release
	// function after use.
	NewPinnedClientDbExecutor(ctx context.Context, organizationId uuid.UUID) (repositories.Executor, func(), error)
}
