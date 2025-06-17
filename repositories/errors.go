package repositories

import (
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
)

func IsUniqueViolationError(err error) bool {
	var pgxErr *pgconn.PgError
	return errors.As(err, &pgxErr) && pgxErr.Code == pgerrcode.UniqueViolation
}

func IsDeadlockError(err error) bool {
	var pgxErr *pgconn.PgError
	return errors.As(err, &pgxErr) && pgxErr.Code == pgerrcode.DeadlockDetected
}

func IsSerializationFailureError(err error) bool {
	var pgxErr *pgconn.PgError
	return errors.As(err, &pgxErr) && pgxErr.Code == pgerrcode.SerializationFailure
}
