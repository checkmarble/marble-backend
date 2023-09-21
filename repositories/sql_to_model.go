package repositories

import (
	"fmt"
	"reflect"

	"github.com/checkmarble/marble-backend/models"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/cockroachdb/errors"
)

func SqlToChannelOfDbModel[DBModel any](tx TransactionPostgres, query squirrel.Sqlizer) (<-chan DBModel, <-chan error) {

	modelsChannel := make(chan DBModel, 100)
	errChannel := make(chan error, 1)

	go func() {
		defer close(modelsChannel)
		defer close(errChannel)

		// var err error
		// for i := 0; i < 1e3; i++ {
		err := ForEachRow(tx, query, func(row pgx.CollectableRow) error {
			dbModel, err := pgx.RowToStructByName[DBModel](row)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("error scanning row to struct %T", dbModel))
			} else {
				modelsChannel <- dbModel
			}
			return nil
		})
		// }
		errChannel <- err

	}()

	return modelsChannel, errChannel
}

func ForEachRow(transaction TransactionPostgres, query squirrel.Sqlizer, fn func(row pgx.CollectableRow) error) error {

	sql, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "can't build sql query")
	}

	rows, err := transaction.exec.Query(transaction.ctx, sql, args...)
	if err != nil {
		return errors.Wrap(err, "error executing sql query")
	}

	defer rows.Close()

	for rows.Next() {
		err := fn(rows)
		if err != nil {
			return err
		}
	}

	return errors.Wrap(rows.Err(), "error iterating over rows")
}

// executes the sql query with the given transaction and returns a list of models using the provided adapter
func SqlToListOfModels[DBModel, Model any](transaction TransactionPostgres, query squirrel.Sqlizer, adapter func(dbModel DBModel) Model) ([]Model, error) {
	return SqlToListOfModelsAdapterWithErr(transaction, query, func(dbModel DBModel) (Model, error) {
		return adapter(dbModel), nil
	})
}

// executes the sql query with the given transaction and returns a models using the provided adapter
// If no result is returned by the query, returns nil
func SqlToOptionalModel[DBModel, Model any](transaction TransactionPostgres, s squirrel.Sqlizer, adapter func(dbModel DBModel) Model) (*Model, error) {

	return SqlToOptionalModelAdapterWithErr(transaction, s, func(dbModel DBModel) (Model, error) {
		return adapter(dbModel), nil
	})
}

// executes the sql query with the given transaction and returns a models using the provided adapter
// if no result is returned by the query, returns a NotFoundError
func SqlToModel[DBModel, Model any](transaction TransactionPostgres, s squirrel.Sqlizer, adapter func(dbModel DBModel) Model) (Model, error) {

	return SqlToModelAdapterWithErr(transaction, s, func(dbModel DBModel) (Model, error) {
		return adapter(dbModel), nil
	})
}

////////////////
// Below, copies of the same functions usable if the dto adapter can return an error (for instance, if it involves unmarshalling a json string)
////////////////

// executes the sql query with the given transaction and returns a list of models using the provided adapter
func SqlToListOfModelsAdapterWithErr[DBModel, Model any](transaction TransactionPostgres, query squirrel.Sqlizer, adapter func(dbModel DBModel) (Model, error)) ([]Model, error) {

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "can't build sql query")
	}

	rows, err := transaction.exec.Query(transaction.ctx, sql, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing sql query")
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Model, error) {
		dbModel, err := pgx.RowToStructByName[DBModel](row)
		if err != nil {
			var zeroModel Model
			return zeroModel, errors.Wrap(err, fmt.Sprintf("error scanning row to struct %T", dbModel))
		}
		return adapter(dbModel)
	})
}

// executes the sql query with the given transaction and returns a models using the provided adapter
// If no result is returned by the query, returns nil
func SqlToOptionalModelAdapterWithErr[DBModel, Model any](transaction TransactionPostgres, s squirrel.Sqlizer, adapter func(dbModel DBModel) (Model, error)) (*Model, error) {

	modelslist, err := SqlToListOfModelsAdapterWithErr(transaction, s, adapter)
	if err != nil {
		return nil, err
	}

	numberOfTesults := len(modelslist)
	if numberOfTesults == 0 {
		return nil, nil
	}
	var model Model = modelslist[0]
	if numberOfTesults > 1 {
		return nil, errors.New(fmt.Sprintf("except 1 or 0 %v, %d rows in the result", reflect.TypeOf(model), numberOfTesults))
	}
	return &model, nil
}

// executes the sql query with the given transaction and returns a models using the provided adapter
// if no result is returned by the query, returns a NotFoundError
func SqlToModelAdapterWithErr[DBModel, Model any](transaction TransactionPostgres, s squirrel.Sqlizer, adapter func(dbModel DBModel) (Model, error)) (Model, error) {

	model, err := SqlToOptionalModelAdapterWithErr(transaction, s, adapter)
	var zeroModel Model
	if err != nil {
		return zeroModel, err
	}
	if model == nil {
		return zeroModel, errors.Wrap(models.NotFoundError, fmt.Sprintf("found no object of type %T", zeroModel))
	}
	return *model, nil
}
