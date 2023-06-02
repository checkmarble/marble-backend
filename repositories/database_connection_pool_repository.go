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
	clientConnectionStrings     map[models.DatabaseName]string
	marbleConnectionPool        *pgxpool.Pool
	clientsConnectionPools      map[models.DatabaseName]*pgxpool.Pool
	clientsConnectionPoolsMutex sync.Mutex
}

func NewDatabaseConnectionPoolRepository(marbleConnectionPool *pgxpool.Pool, clientConnectionStrings map[models.DatabaseName]string) DatabaseConnectionPoolRepository {

	return &DatabaseConnectionPoolRepositoryImpl{
		clientConnectionStrings: clientConnectionStrings,
		marbleConnectionPool:    marbleConnectionPool,
		clientsConnectionPools:  make(map[models.DatabaseName]*pgxpool.Pool),
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
		if pool, found := repo.clientsConnectionPools[db.Name]; found {
			return pool, nil
		}

		// create and register new pool
		newPool, err := repo.newClientDatabaseConnectionPool(db.Name)
		if err != nil {
			return nil, err
		}
		repo.clientsConnectionPools[db.Name] = newPool
		return newPool, nil
	}

	return nil, errors.New("DatabaseConnectionPoolRepositoryImpl: unknown database type")
}

func (repo *DatabaseConnectionPoolRepositoryImpl) newClientDatabaseConnectionPool(name models.DatabaseName) (*pgxpool.Pool, error) {

	connectionString, ok := repo.clientConnectionStrings[name]
	if !ok {
		return nil, fmt.Errorf("DatabaseConnectionPoolRepositoryImpl: unknown database name: %s", name)
	}

	dbpool, err := pgxpool.New(context.Background(), connectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return dbpool, nil
}
