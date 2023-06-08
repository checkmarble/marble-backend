package repositories

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseConnectionPoolRepository interface {
	DatabaseConnectionPool(connection models.PostgresConnection) (*pgxpool.Pool, error)
}

type DatabaseConnectionPoolRepositoryImpl struct {
	marbleConnectionPool        *pgxpool.Pool
	clientsConnectionPools      map[models.PostgresConnection]*pgxpool.Pool
	clientsConnectionPoolsMutex sync.Mutex
}

func NewDatabaseConnectionPoolRepository(marbleConnectionPool *pgxpool.Pool) DatabaseConnectionPoolRepository {

	repo := &DatabaseConnectionPoolRepositoryImpl{
		marbleConnectionPool:   marbleConnectionPool,
		clientsConnectionPools: make(map[models.PostgresConnection]*pgxpool.Pool),
	}
	repo.clientsConnectionPools[models.DATABASE_MARBLE.Connection] = marbleConnectionPool
	return repo
}

func (repo *DatabaseConnectionPoolRepositoryImpl) DatabaseConnectionPool(connection models.PostgresConnection) (*pgxpool.Pool, error) {

	repo.clientsConnectionPoolsMutex.Lock()
	defer repo.clientsConnectionPoolsMutex.Unlock()

	// return existing pool is already created
	if pool, found := repo.clientsConnectionPools[connection]; found {
		return pool, nil
	}

	// create and register new pool
	newPool, err := repo.newClientDatabaseConnectionPool(connection)
	if err != nil {
		return nil, err
	}
	repo.clientsConnectionPools[connection] = newPool
	return newPool, nil
}

func (repo *DatabaseConnectionPoolRepositoryImpl) newClientDatabaseConnectionPool(connection models.PostgresConnection) (*pgxpool.Pool, error) {

	dbpool, err := pgxpool.New(context.Background(), string(connection))
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return dbpool, nil
}
