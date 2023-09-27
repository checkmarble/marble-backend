package repositories

import (
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/cockroachdb/errors"
)

func FuncReturnsNilError[DBModel any, Model any](fn func(db DBModel) Model) func(db DBModel) (Model, error) {
	return func(db DBModel) (Model, error) {
		return fn(db), nil
	}
}

func adaptModelUsingRowToStruct[DBModel any, Model any](row pgx.CollectableRow, adapter func(dbModel DBModel) (Model, error)) (Model, error) {

	dbModel, err := pgx.RowToStructByName[DBModel](row)
	if err != nil {
		var zeroModel Model
		return zeroModel, errors.Wrap(err, fmt.Sprintf("error scanning row to struct %T", dbModel))
	}
	return adapter(dbModel)
}

// executes the sql query with the given transaction and returns a list of models using the provided adapter
func SqlToListOfModels[DBModel, Model any](transaction TransactionPostgres, query squirrel.Sqlizer, adapter func(dbModel DBModel) (Model, error)) ([]Model, error) {

	return SqlToListOfRow(transaction, query, func(row pgx.CollectableRow) (Model, error) {
		return adaptModelUsingRowToStruct(row, adapter)
	})
}

// executes the sql query with the given transaction and returns a models using the provided adapter
// If no result is returned by the query, returns nil
func SqlToOptionalModel[DBModel, Model any](transaction TransactionPostgres, s squirrel.Sqlizer, adapter func(dbModel DBModel) (Model, error)) (*Model, error) {

	return SqlToOptionalRow(transaction, s, func(row pgx.CollectableRow) (Model, error) {
		return adaptModelUsingRowToStruct(row, adapter)
	})
}

// executes the sql query with the given transaction and returns a models using the provided adapter
// if no result is returned by the query, returns a NotFoundError
func SqlToModel[DBModel, Model any](transaction TransactionPostgres, s squirrel.Sqlizer, adapter func(dbModel DBModel) (Model, error)) (Model, error) {

	return SqlToRow(transaction, s, func(row pgx.CollectableRow) (Model, error) {
		return adaptModelUsingRowToStruct(row, adapter)
	})
}
