package repositories

import (
	"fmt"
	"marble/marble-backend/models"
	"reflect"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

// executes the sql query with the given transaction and returns a list of models using the provided adapter
func SqlToListOfModels[DBModel, Model any](transaction TransactionPostgres, s squirrel.SelectBuilder, adapter func(dbModel DBModel) Model) ([]Model, error) {

	query, args, err := s.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := transaction.exec.Query(transaction.ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Model, error) {
		dbModel, err := pgx.RowToStructByName[DBModel](row)
		if err != nil {
			var zeroModel Model
			return zeroModel, err
		}
		return adapter(dbModel), nil
	})
}

// executes the sql query with the given transaction and returns a models using the provided adapter
// If no result is returned by the query, returns nil
func SqlToOptionalModel[DBModel, Model any](transaction TransactionPostgres, s squirrel.SelectBuilder, adapter func(dbModel DBModel) Model) (*Model, error) {

	modelslist, err := SqlToListOfModels(transaction, s, adapter)
	if err != nil {
		return nil, err
	}

	numberOfTesults := len(modelslist)
	if numberOfTesults == 0 {
		return nil, nil
	}
	var model Model = modelslist[0]
	if numberOfTesults > 1 {
		return nil, fmt.Errorf("except 1 or 0 %v, %d rows in the result", reflect.TypeOf(model), numberOfTesults)
	}
	return &model, nil
}

// executes the sql query with the given transaction and returns a models using the provided adapter
// if no result is returned by the query, returns a NotFoundError
func SqlToModel[DBModel, Model any](transaction TransactionPostgres, s squirrel.SelectBuilder, adapter func(dbModel DBModel) Model) (Model, error) {

	model, err := SqlToOptionalModel(transaction, s, adapter)
	var zeroModel Model
	if err != nil {
		return zeroModel, err
	}
	if model == nil {
		return zeroModel, fmt.Errorf("%v %w", reflect.TypeOf(zeroModel).Name(), models.NotFoundError)
	}
	return *model, nil
}
