package repositories

import "context"

func (repo *MarbleDbRepository) Liveness(ctx context.Context, exec Executor) error {
	sql := "SELECT 1"
	row := exec.QueryRow(ctx, sql)
	var result int
	if err := row.Scan(&result); err != nil {
		return err
	}
	return nil
}
