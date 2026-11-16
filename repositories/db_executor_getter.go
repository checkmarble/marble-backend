package repositories

import (
	"context"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/cockroachdb/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultOrgConfigKey   = "default"
	clientDbWarmupTimeout = 30 * time.Second
)

type ExecutorGetter struct {
	appName              string
	marbleConnectionPool *pgxpool.Pool
	redisClient          *RedisClient

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

type cacheGetter interface {
	Cache(context.Context) *RedisExecutor
}

type Executor interface {
	TransactionOrPool
	databaseSchemaGetter
	cacheGetter
}

type Transaction interface {
	databaseSchemaGetter
	cacheGetter
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	RawTx() pgx.Tx
	Begin(ctx context.Context) (Transaction, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

func NewExecutorGetter(
	pool *pgxpool.Pool,
	clientDbConfigs map[string]infra.ClientDbConfig,
	redisClient *RedisClient,
	tp trace.TracerProvider,
) ExecutorGetter {
	return ExecutorGetter{
		clientDbConfigs:      clientDbConfigs,
		redisClient:          redisClient,
		clientDbPools:        make(map[string]*pgxpool.Pool, len(clientDbConfigs)),
		marbleConnectionPool: pool,
		tp:                   tp,
		mu:                   &sync.Mutex{},
	}
}

func (g ExecutorGetter) Transaction(
	ctx context.Context,
	typ models.DatabaseSchemaType,
	org *models.Organization,
	fn func(exec Transaction) error,
) error {
	pool, databaseSchema, err := g.getPoolAndSchema(ctx, typ, org)
	if err != nil {
		return errors.Wrap(err, "Error getting pool and schema")
	}

	execInTransaction := func() error {
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := injectDbSessionConfig(ctx, tx, ""); err != nil {
				return err
			}

			orgId := uuid.Nil
			if org != nil {
				orgId = org.Id
			}

			return fn(&PgTx{
				databaseSchema: databaseSchema,
				tx:             tx,
				cache:          g.redisClient.NewExecutor(orgId),
			})
		})
	}
	// Retry retryable commit errors once, immediately after failure.
	err = retry.Do(execInTransaction,
		retry.Attempts(2),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			return IsDeadlockError(err) || IsSerializationFailureError(err)
		}),
		retry.OnRetry(func(_ uint, err error) {
			logger := utils.LoggerFromContext(ctx).With("database_schema", databaseSchema.Schema)
			switch {
			case IsDeadlockError(err):
				logger.WarnContext(ctx, "Deadlock detected, retrying transaction")
			case IsSerializationFailureError(err):
				logger.WarnContext(ctx, "Serialization failure detected, retrying transaction")
			}
		}),
	)

	return errors.Wrap(err, "Error executing transaction")
}

func (g ExecutorGetter) getPoolAndSchema(
	ctx context.Context,
	typ models.DatabaseSchemaType,
	org *models.Organization,
) (*pgxpool.Pool, models.DatabaseSchema, error) {
	// For a marble connection pool, just use the existing pool
	if typ == models.DATABASE_SCHEMA_TYPE_MARBLE {
		return g.marbleConnectionPool, models.DATABASE_MARBLE_SCHEMA, nil
	}

	if org == nil {
		return nil, models.DatabaseSchema{}, errors.New(
			"Organization must be provided for client database")
	}

	// For a client connection pool, create a new pool if it doesn't exist. Several customers can share the same pool, depending on the config.
	config, ok := g.clientDbConfigs[org.Id.String()]
	// if no specific DB is configured for the client, put the data in a dedicated schema in the main marble DB
	if !ok {
		// If no specific DB is configured for the client, check if there is a default config to use instead for all orgs
		defaultConfig, defaultConfigFound := g.clientDbConfigs[defaultOrgConfigKey]
		if defaultConfigFound {
			config = defaultConfig
			config.SchemaName = ""
		} else {
			// as fallback, use the marble db with the default schema name
			return g.marbleConnectionPool, models.DatabaseSchema{
				SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
				Schema:     models.OrgSchemaName(org.Name),
			}, nil
		}
	}

	if config.ConnectionString == "" {
		return nil, models.DatabaseSchema{}, errors.New(
			"Client DB config must have a connection string")
	}
	if config.SchemaName == "" {
		config.SchemaName = models.OrgSchemaName(org.Name)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	pool, ok := g.clientDbPools[config.ConnectionString]
	if !ok {
		var err error
		pool, err = g.newClientDbPool(ctx, config)
		if err != nil {
			return nil, models.DatabaseSchema{}, err
		}

		g.clientDbPools[config.ConnectionString] = pool
	}

	return pool, models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Schema:     config.SchemaName,
	}, nil
}

func (g ExecutorGetter) newClientDbPool(ctx context.Context, config infra.ClientDbConfig) (*pgxpool.Pool, error) {
	pool, err := infra.NewPostgresConnectionPool(ctx, g.appName, config.ConnectionString, g.tp, infra.DEFAULT_CLIENTDB_MIN_CONNECTIONS, config.MaxConns, config.ImpersonateRole)
	if err != nil {
		// A pool that failed its initial ping would otherwise keep trying to open its minimum connections forever.
		if pool != nil {
			pool.Close()
		}

		return nil, errors.Wrap(err, "Error creating connection pool")
	}

	return pool, nil
}

// WarmupClientDbPools opens every configured client database pool along with its minimum connections, so the
// first request to each organization does not pay for the connection setup. A database that cannot be reached
// is only logged, and will be retried lazily on its first use. It must be called before the executor getter is
// used, since it overwrites any pool already registered for the same connection string.
func (g ExecutorGetter) WarmupClientDbPools(ctx context.Context) {
	configs := make(map[string]infra.ClientDbConfig, len(g.clientDbConfigs))
	connectionStrings := make(map[string]struct{}, len(g.clientDbConfigs))

	for configKey, config := range g.clientDbConfigs {
		if config.ConnectionString == "" {
			continue
		}

		if _, ok := connectionStrings[config.ConnectionString]; ok {
			continue
		}

		connectionStrings[config.ConnectionString] = struct{}{}
		configs[configKey] = config
	}

	if len(configs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, clientDbWarmupTimeout)
	defer cancel()

	logger := utils.LoggerFromContext(ctx)

	var wg sync.WaitGroup

	for configKey, config := range configs {
		wg.Go(func() {
			if err := g.warmupClientDbPool(ctx, configKey, config); err != nil {
				logger.WarnContext(ctx, "could not warm up client database connection pool",
					"org_id", configKey,
					"error", err.Error(),
				)
			}
		})
	}

	wg.Wait()
}

func (g ExecutorGetter) warmupClientDbPool(ctx context.Context, configKey string, config infra.ClientDbConfig) error {
	pool, err := g.newClientDbPool(ctx, config)
	if err != nil {
		return err
	}

	g.mu.Lock()
	g.clientDbPools[config.ConnectionString] = pool
	g.mu.Unlock()

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for pool.Stat().IdleConns() < pool.Config().MinConns {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "Error opening minimum connections")
		case <-ticker.C:
		}
	}

	connConfig := pool.Config().ConnConfig

	utils.LoggerFromContext(ctx).DebugContext(ctx, "established client database connections",
		"org_id", configKey,
		"database", connConfig.Database,
	)

	return nil
}

func (g ExecutorGetter) GetExecutor(
	ctx context.Context,
	typ models.DatabaseSchemaType,
	org *models.Organization,
) (Executor, error) {
	pool, databaseSchema, err := g.getPoolAndSchema(ctx, typ, org)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting pool and schema")
	}

	orgId := uuid.Nil
	if org != nil {
		orgId = org.Id
	}

	return &PgExecutor{
		databaseSchema: databaseSchema,
		exec:           pool,
		cache:          g.redisClient.NewExecutor(orgId),
	}, nil
}

func (g ExecutorGetter) GetPinnedExecutor(
	ctx context.Context,
	typ models.DatabaseSchemaType,
	org *models.Organization,
) (Executor, func(), error) {
	pool, databaseSchema, err := g.getPoolAndSchema(ctx, typ, org)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error getting pool and schema")
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error acquiring connection from pool")
	}

	return &PgExecutor{
		databaseSchema: databaseSchema,
		exec:           conn,
	}, func() {
		conn.Release()
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
