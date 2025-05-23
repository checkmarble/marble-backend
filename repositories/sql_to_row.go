package repositories

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
)

func SqlToChannelOfModels[Model any](ctx context.Context, exec Executor, query squirrel.Sqlizer,
	adapter func(row pgx.CollectableRow) (Model, error),
) (<-chan Model, <-chan error) {
	modelsChannel := make(chan Model, 100)
	errChannel := make(chan error, 1)

	go func() {
		defer close(modelsChannel)
		defer close(errChannel)

		err := ForEachRow(ctx, exec, query, func(row pgx.CollectableRow) error {
			model, err := adapter(row)
			if err != nil {
				return err
			} else {
				modelsChannel <- model
			}
			return nil
		})
		errChannel <- err
	}()

	return modelsChannel, errChannel
}

type ModelResult[M any] struct {
	Model M
	Error error
}

func SqlToFallibleChannelOfModel[Model any](ctx context.Context, exec Executor, query squirrel.Sqlizer,
	adapter func(row pgx.CollectableRow) (Model, error),
) <-chan ModelResult[Model] {
	modelsChannel := make(chan ModelResult[Model], 1)

	go func() {
		defer close(modelsChannel)

		err := ForEachRow(ctx, exec, query, func(row pgx.CollectableRow) error {
			model, err := adapter(row)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				modelsChannel <- ModelResult[Model]{model, err}
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			select {
			case <-ctx.Done():
				return
			default:
				modelsChannel <- ModelResult[Model]{*new(Model), err}
			}
			return
		}
	}()

	return modelsChannel
}

func SqlToListOfRow[Model any](ctx context.Context, exec Executor, query squirrel.Sqlizer,
	adapter func(row pgx.CollectableRow) (Model, error),
) ([]Model, error) {
	models := make([]Model, 0)
	err := ForEachRow(ctx, exec, query, func(row pgx.CollectableRow) error {
		model, err := adapter(row)
		if err != nil {
			return err
		}
		models = append(models, model)
		return nil
	})
	return models, err
}

func SqlToOptionalRow[Model any](ctx context.Context, exec Executor, s squirrel.Sqlizer,
	adapter func(row pgx.CollectableRow) (Model, error),
) (*Model, error) {
	models, err := SqlToListOfRow(ctx, exec, s, adapter)
	if err != nil {
		return nil, err
	}

	numberOfResults := len(models)
	if numberOfResults == 0 {
		return nil, nil
	}

	model := models[0]
	if numberOfResults > 1 {
		return nil, errors.New(fmt.Sprintf("expect 1 or 0 %v, %d rows in the result",
			reflect.TypeOf(model), numberOfResults))
	}
	return &model, nil
}

func SqlToRow[Model any](ctx context.Context, exec Executor, s squirrel.Sqlizer,
	adapter func(row pgx.CollectableRow) (Model, error),
) (Model, error) {
	model, err := SqlToOptionalRow(ctx, exec, s, adapter)
	var zeroModel Model
	if err != nil {
		return zeroModel, err
	}
	if model == nil {
		return zeroModel, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("found no object of type %T", zeroModel))
	}
	return *model, nil
}

func ForEachRow(ctx context.Context, exec Executor, query squirrel.Sqlizer, fn func(row pgx.CollectableRow) error) error {
	sql, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "can't build sql query")
	}

	rows, err := exec.Query(ctx, sql, args...)
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

type SqlBuilder interface {
	ToSql() (string, []interface{}, error)
}

func ExecBuilder(ctx context.Context, exec Executor, builder SqlBuilder) (err error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return errors.Wrap(err, "can't build sql query")
	}

	_, err = exec.Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error executing sql query: %s", query))
	}
	return nil
}
