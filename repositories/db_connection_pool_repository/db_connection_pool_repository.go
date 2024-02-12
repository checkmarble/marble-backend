package db_connection_pool_repository

import (
	"context"
	"sync"

	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseConnectionPoolRepository struct {
	marbleConnectionPool        *pgxpool.Pool
	clientsConnectionPools      map[models.PostgresConnection]*pgxpool.Pool
	clientsConnectionPoolsMutex sync.Mutex
}

func NewDatabaseConnectionPoolRepository(marbleConnectionPool *pgxpool.Pool) *DatabaseConnectionPoolRepository {

	repo := &DatabaseConnectionPoolRepository{
		marbleConnectionPool:   marbleConnectionPool,
		clientsConnectionPools: make(map[models.PostgresConnection]*pgxpool.Pool),
	}
	repo.clientsConnectionPools[models.DATABASE_MARBLE.Connection] = marbleConnectionPool
	return repo
}

func (repo *DatabaseConnectionPoolRepository) DatabaseConnectionPool(ctx context.Context, connection models.PostgresConnection) (*pgxpool.Pool, error) {

	repo.clientsConnectionPoolsMutex.Lock()
	defer repo.clientsConnectionPoolsMutex.Unlock()

	// return existing pool is already created
	if pool, found := repo.clientsConnectionPools[connection]; found {
		return pool, nil
	}

	// create and register new pool
	newPool, err := repo.newClientDatabaseConnectionPool(ctx, connection)
	if err != nil {
		return nil, err
	}
	repo.clientsConnectionPools[connection] = newPool
	return newPool, nil
}

func (repo *DatabaseConnectionPoolRepository) newClientDatabaseConnectionPool(ctx context.Context, connection models.PostgresConnection) (*pgxpool.Pool, error) {

	dbpool, err := pgxpool.New(ctx, string(connection))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create connection pool")
	}

	return dbpool, nil
}
