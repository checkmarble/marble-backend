package pg_repository

import (
	"marble/marble-backend/app"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func GetDbField(path []string, fieldName string, dataModel app.DataModel, payload app.Payload) (interface{}, error) {

	return nil, nil
}
