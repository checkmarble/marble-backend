package repositories

import (
	"fmt"
	"reflect"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
)

func SqlToChannelOfModels[Model any](tx TransactionPostgres, query squirrel.Sqlizer, adapter func(row pgx.CollectableRow) (Model, error)) (<-chan Model, <-chan error) {

	modelsChannel := make(chan Model, 100)
	errChannel := make(chan error, 1)

	go func() {
		defer close(modelsChannel)
		defer close(errChannel)

		// var err error
		// for i := 0; i < 1e3; i++ {
		err := ForEachRow(tx, query, func(row pgx.CollectableRow) error {
			model, err := adapter(row)
			if err != nil {
				return err
			} else {
				modelsChannel <- model
			}
			return nil
		})
		// }
		errChannel <- err

	}()

	return modelsChannel, errChannel
}

func SqlToListOfRow[Model any](tx TransactionPostgres, query squirrel.Sqlizer, adapter func(row pgx.CollectableRow) (Model, error)) ([]Model, error) {

	models := make([]Model, 0)
	err := ForEachRow(tx, query, func(row pgx.CollectableRow) error {
		model, err := adapter(row)
		if err == nil {
			models = append(models, model)
		}
		return err
	})

	if err != nil {
		return nil, err
	}
	return models, nil
}

func SqlToOptionalRow[Model any](transaction TransactionPostgres, s squirrel.Sqlizer, adapter func(row pgx.CollectableRow) (Model, error)) (*Model, error) {

	models, err := SqlToListOfRow(transaction, s, adapter)
	if err != nil {
		return nil, err
	}

	numberOfTesults := len(models)
	if numberOfTesults == 0 {
		return nil, nil
	}

	var model Model = models[0]
	if numberOfTesults > 1 {
		return nil, errors.New(fmt.Sprintf("except 1 or 0 %v, %d rows in the result", reflect.TypeOf(model), numberOfTesults))
	}
	return &model, nil
}

func SqlToRow[Model any](transaction TransactionPostgres, s squirrel.Sqlizer, adapter func(row pgx.CollectableRow) (Model, error)) (Model, error) {

	model, err := SqlToOptionalRow(transaction, s, adapter)
	var zeroModel Model
	if err != nil {
		return zeroModel, err
	}
	if model == nil {
		return zeroModel, errors.Wrap(models.NotFoundError, fmt.Sprintf("found no object of type %T", zeroModel))
	}
	return *model, nil

}

func ForEachRow(transaction TransactionPostgres, query squirrel.Sqlizer, fn func(row pgx.CollectableRow) error) error {

	sql, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "can't build sql query")
	}

	rows, err := transaction.exec.Query(transaction.ctx, sql, args...)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error executing sql query: %s", sql))
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
