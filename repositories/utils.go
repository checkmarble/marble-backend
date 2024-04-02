package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

const postgres_audit_user_id_parameter = "custom.current_user_id"

func setCurrentUserIdContext(ctx context.Context, exec Executor, userId *models.UserId) error {
	if userId != nil {
		_, err := exec.Exec(
			ctx,
			fmt.Sprintf("SELECT SET_CONFIG('%s', $1, false)", postgres_audit_user_id_parameter),
			*userId,
		)
		return err
	}
	return nil
}

func columnsNames(tablename string, fields []string) []string {
	return pure_utils.Map(fields, func(f string) string {
		return fmt.Sprintf("%s.%s", tablename, f)
	})
}
