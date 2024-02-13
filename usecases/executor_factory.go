package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/repositories"
)

// Interface to be used in usecases, implemented by the DbExecutorFactory class in the usecases/db_executor_factory package
// which itself has the ExecutorGetter repository class injected in it.
type ClientSchemaExecutorFactory interface {
	NewClientDbExecutor(ctx context.Context, organizationId string) (repositories.Executor, error)
}
