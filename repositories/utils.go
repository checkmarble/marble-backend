package repositories

import (
	"fmt"

	"github.com/checkmarble/marble-backend/utils"
)

func columnsNames(tablename string, fields []string) []string {
	return utils.Map(fields, func(f string) string {
		return fmt.Sprintf("%s.%s", tablename, f)
	})
}
