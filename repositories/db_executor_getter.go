package repositories

import (
	"context"
	"sync"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"go.opentelemetry.io/otel/trace"

	"github.com/cockroachdb/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutorGetter struct {
	marbleConnectionPool *pgxpool.Pool

	// uses the organizationId as the key
	clientDbConfigs map[string]infra.ClientDbConfig

	// uses the connection string as a key
	clientDbPools map[string]*pgxpool.Pool
	// used to make the clientDbPools map thread-safe
	mu *sync.Mutex

	tp trace.TracerProvider
}

type databaseSchemaGetter interface {
	DatabaseSchema() models.DatabaseSchema
}

type Executor interface {
	TransactionOrPool
	databaseSchemaGetter
}

type Transaction interface {
	databaseSchemaGetter
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	RawTx() pgx.Tx
}

func NewExecutorGetter(pool *pgxpool.Pool) ExecutorGetter {
	return ExecutorGetter{
		marbleConnectionPool: pool,
		// Add the other fields
	}
}

func (g ExecutorGetter) Transaction(
	ctx context.Context,
	typ models.DatabaseSchemaType,
	organizationId string,
	organizationName string,
	fn func(exec Transaction) error,
) error {
	pool, databaseSchema, err := g.getPoolAndSchema(ctx, typ, organizationId, organizationName)
	if err != nil {
		return errors.Wrap(err, "Error getting pool and schema")
	}

	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return fn(&PgTx{
			databaseSchema: databaseSchema,
			tx:             tx,
		})
	})

	return errors.Wrap(err, "Error executing transaction")
}

func (g ExecutorGetter) getPoolAndSchema(
	ctx context.Context,
	typ models.DatabaseSchemaType,
	organizationId string,
	organizationName string,
) (*pgxpool.Pool, models.DatabaseSchema, error) {
	// For a marble connection pool, just use the existing pool
	if typ == models.DATABASE_SCHEMA_TYPE_MARBLE {
		return g.marbleConnectionPool, models.DATABASE_MARBLE_SCHEMA, nil
	}

	// For a client connection pool, create a new pool if it doesn't exist. Several customers can share the same pool, depending on the config.
	config, ok := g.clientDbConfigs[organizationId]
	// if no specific DB is configured for the client, put the data in a dedicated schema in the main marble DB
	if !ok {
		return g.marbleConnectionPool, models.DatabaseSchema{
			SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
			Schema:     models.OrgSchemaName(organizationName),
		}, nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	pool, ok := g.clientDbPools[config.ConnectionString]
	if !ok {
		var err error
		pool, err = infra.NewPostgresConnectionPool(
			ctx,
			config.ConnectionString,
			g.tp,
			config.MaxConns,
		)
		if err != nil {
			return nil, models.DatabaseSchema{}, errors.Wrap(err, "Error creating connection pool")
		}
		g.clientDbPools[config.ConnectionString] = pool
	}

	return pool, models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Schema:     config.SchemaName,
	}, nil
}

func (g ExecutorGetter) GetExecutor(
	ctx context.Context,
	typ models.DatabaseSchemaType,
	organizationId string,
	organizationName string,
) (Executor, error) {
	pool, databaseSchema, err := g.getPoolAndSchema(ctx, typ, organizationId, organizationName)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting pool and schema")
	}

	return &PgExecutor{
		databaseSchema: databaseSchema,
		exec:           pool,
	}, nil
}

func validateClientDbExecutor(exec databaseSchemaGetter) error {
	if exec == nil {
		return errors.New("Cannot use nil executor for client database")
	}
	if exec.DatabaseSchema().SchemaType != models.DATABASE_SCHEMA_TYPE_CLIENT {
		return errors.New("Cannot use marble db executor to query client database")
	}
	return nil
}

func validateMarbleDbExecutor(exec databaseSchemaGetter) error {
	if exec == nil {
		return errors.New("Cannot use nil executor for marble database")
	}
	if exec.DatabaseSchema().SchemaType != models.DATABASE_SCHEMA_TYPE_MARBLE {
		return errors.New("Cannot use client db executor to query marble database")
	}
	return nil
}
