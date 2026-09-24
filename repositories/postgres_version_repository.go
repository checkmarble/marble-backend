package repositories

import (
	"context"

	"github.com/cockroachdb/errors"
)

func (repo MarbleDbRepository) GetPostgresVersion(ctx context.Context, exec Executor) (int, error) {
	row := exec.QueryRow(ctx, "select current_setting('server_version_num')::int / 10000 as pg_version")

	var version int

	if err := row.Scan(&version); err != nil {
		return 0, errors.Wrap(err, "could not scan postgres version")
	}

	return version, nil
}
