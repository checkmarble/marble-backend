package repositories

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseConnectionPoolRepository interface {
	DatabaseConnectionPool(db models.Database) (*pgxpool.Pool, error)
}

type DatabaseConnectionPoolRepositoryImpl struct {
	marbleConnectionPool        *pgxpool.Pool
	clientsConnectionPools      map[models.PostgresConnection]*pgxpool.Pool
	clientsConnectionPoolsMutex sync.Mutex
}

func NewDatabaseConnectionPoolRepository(marbleConnectionPool *pgxpool.Pool) DatabaseConnectionPoolRepository {

	return &DatabaseConnectionPoolRepositoryImpl{
		marbleConnectionPool:   marbleConnectionPool,
		clientsConnectionPools: make(map[models.PostgresConnection]*pgxpool.Pool),
	}
}

func (repo *DatabaseConnectionPoolRepositoryImpl) DatabaseConnectionPool(db models.Database) (*pgxpool.Pool, error) {
	if db.DatabaseType == models.DATABASE_TYPE_MARBLE {
		return repo.marbleConnectionPool, nil
	}

	if db.DatabaseType == models.DATABASE_TYPE_CLIENT {

		repo.clientsConnectionPoolsMutex.Lock()
		defer repo.clientsConnectionPoolsMutex.Unlock()

		// return existing pool is already created
		if pool, found := repo.clientsConnectionPools[db.Connection]; found {
			return pool, nil
		}

		// create and register new pool
		newPool, err := repo.newClientDatabaseConnectionPool(db.Connection)
		if err != nil {
			return nil, err
		}
		repo.clientsConnectionPools[db.Connection] = newPool
		return newPool, nil
	}

	return nil, errors.New("DatabaseConnectionPoolRepositoryImpl: unknown database type")
}

func (repo *DatabaseConnectionPoolRepositoryImpl) newClientDatabaseConnectionPool(connection models.PostgresConnection) (*pgxpool.Pool, error) {

	dbpool, err := pgxpool.New(context.Background(), string(connection))
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return dbpool, nil
}
