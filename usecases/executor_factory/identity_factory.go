package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/cockroachdb/errors"
)

// IdentityExecutorFactory will only ever create two specific executors:
//   - An already started transaction for the Marble database
//   - An ingested data database for a given organization
type IdentityExecutorFactory struct {
	marbleExec repositories.Transaction
	getter     repositories.ExecutorGetter
	org        models.Organization
}

func NewIdentityExecutorFactory(marbleExec repositories.Transaction, getter repositories.ExecutorGetter, org models.Organization) IdentityExecutorFactory {
	return IdentityExecutorFactory{
		marbleExec: marbleExec,
		getter:     getter,
		org:        org,
	}
}

func (f IdentityExecutorFactory) NewExecutor() repositories.Executor {
	return f.marbleExec
}

func (f IdentityExecutorFactory) NewClientDbExecutor(ctx context.Context, orgId string) (repositories.Executor, error) {
	if orgId != f.org.Id {
		return nil, errors.Newf("IdentityExecutorFactory was built for organization %s but used for organization %s", f.org.Id, orgId)
	}

	return f.getter.GetExecutor(
		ctx,
		models.DATABASE_SCHEMA_TYPE_CLIENT,
		&f.org,
	)
}

func (f IdentityExecutorFactory) Transaction(ctx context.Context, cb func(tx repositories.Transaction) error) error {
	return cb(f.marbleExec)
}

func (f IdentityExecutorFactory) TransactionInOrgSchema(ctx context.Context, orgId string, cb func(tx repositories.Transaction) error) error {
	if orgId != f.org.Id {
		return errors.Newf("IdentityExecutorFactory was built for organization %s but used for organization %s", f.org.Id, orgId)
	}

	return f.getter.Transaction(ctx, models.DATABASE_SCHEMA_TYPE_CLIENT, &f.org, cb)
}
