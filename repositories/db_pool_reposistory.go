package repositories

import "github.com/jackc/pgx/v5/pgxpool"

type DbPoolRepository interface {
	GetDbPool() *pgxpool.Pool
}
