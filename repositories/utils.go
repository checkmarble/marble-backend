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

func columnsNamesWithAlias(tablename string, fields []string) []string {
	return utils.Map(fields, func(f string) string {
		return fmt.Sprintf("%s.%s AS %s_%s", tablename, f, tablename, f)
	})
}

func columnsAlias(tablename string, fields []string) []string {
	return utils.Map(fields, func(f string) string {
		return fmt.Sprintf("%s_%s", tablename, f)
	})
}
