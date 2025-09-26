package repositories

import (
	"context"
	"hash/fnv"
)

func hashStringToBigInt(s string) (int64, error) {
	hash := fnv.New64()
	_, err := hash.Write([]byte(s))
	if err != nil {
		return 0, err
	}
	return int64(hash.Sum64()), nil
}

// pg_advisory_xact_lock is a transaction-level advisory lock
// cf: https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS-TABLE
func GetAdvisoryLockTx(ctx context.Context, tx Transaction, key string) error {
	// Lock takes a bigint, hash the string
	keyInt, err := hashStringToBigInt(key)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", keyInt)
	if err != nil {
		return err
	}
	return nil
}

// pg_advisory_lock is a session-level advisory lock and could be released manually (call defer on the first return value)
// cf: https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS-TABLE
func GetAdvisoryLock(ctx context.Context, exec Executor, key string) (func() error, error) {
	keyInt, err := hashStringToBigInt(key)
	if err != nil {
		return nil, err
	}
	_, err = exec.Exec(ctx, "SELECT pg_advisory_lock($1)", keyInt)
	if err != nil {
		return nil, err
	}
	return func() error {
		_, err := exec.Exec(ctx, "SELECT pg_advisory_unlock($1)", keyInt)
		return err
	}, nil
}
