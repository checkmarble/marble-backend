package repositories

import (
	"fmt"

	"github.com/checkmarble/marble-backend/pure_utils"
)

func columnsNames(tablename string, fields []string) []string {
	return pure_utils.Map(fields, func(f string) string {
		return fmt.Sprintf("%s.%s", tablename, f)
	})
}
