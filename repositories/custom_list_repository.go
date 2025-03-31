package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"

	"github.com/Masterminds/squirrel"
)

type CustomListRepository interface {
	AllCustomLists(ctx context.Context, exec Executor, organizationId string) ([]models.CustomList, error)
	GetCustomListById(ctx context.Context, exec Executor, id string) (models.CustomList, error)
	GetCustomListValues(
		ctx context.Context,
		exec Executor,
		getCustomList models.GetCustomListValuesInput,
		forUpdate ...bool,
	) ([]models.CustomListValue, error)
	GetCustomListValueById(ctx context.Context, exec Executor, id string) (models.CustomListValue, error)
	CreateCustomList(ctx context.Context, exec Executor, createCustomList models.CreateCustomListInput, newCustomListId string) error
	UpdateCustomList(ctx context.Context, exec Executor, updateCustomList models.UpdateCustomListInput) error
	SoftDeleteCustomList(ctx context.Context, exec Executor, listId string) error
	AddCustomListValue(
		ctx context.Context,
		exec Executor,
		addCustomListValue models.AddCustomListValueInput,
		newCustomListId string,
		userId *models.UserId,
	) error
	BatchInsertCustomListValues(
		ctx context.Context,
		exec Executor,
		customListId string,
		customListValues []models.BatchInsertCustomListValue,
		userId *models.UserId,
	) error
	DeleteCustomListValue(
		ctx context.Context,
		exec Executor,
		deleteCustomListValue models.DeleteCustomListValueInput,
		userId *models.UserId,
	) error
	BatchDeleteCustomListValues(
		ctx context.Context,
		exec Executor,
		customListId string,
		deleteCustomListValueIds []string,
		userId *models.UserId,
	) error
}

type CustomListRepositoryPostgresql struct{}

func (repo *CustomListRepositoryPostgresql) AllCustomLists(
	ctx context.Context,
	exec Executor,
	organizationId string,
) ([]models.CustomList, error) {
	query := `
			SELECT
				cl.*,
				(
					SELECT count(*)
					FROM (
						SELECT *
						FROM custom_list_values AS clv
						WHERE clv.custom_list_id=cl.id
						LIMIT $2
						) as sub
					) as nb_items
			FROM custom_lists AS cl
			WHERE cl.organization_id = $1
				AND cl.deleted_at IS NULL
			ORDER BY cl.name
	`
	rows, err := exec.Query(ctx, query, organizationId, models.VALUES_COUNT_LIMIT+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	customsList, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.CustomList, error) {
		customList, err := adaptModelUsingRowToStruct(row, dbmodels.AdaptCustomList)
		if err != nil {
			return models.CustomList{}, err
		}
		return customList, nil
	})
	if err != nil {
		return nil, err
	}
	return customsList, nil
}

func (repo *CustomListRepositoryPostgresql) GetCustomListById(ctx context.Context, exec Executor, id string) (models.CustomList, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.CustomList{}, err
	}

	query := `
			SELECT
				cl.*,
				(
					SELECT count(*)
					FROM (
						SELECT *
						FROM custom_list_values AS clv
						WHERE clv.custom_list_id=cl.id
						LIMIT $2
						) as sub
					) as nb_items
			FROM custom_lists AS cl
			WHERE cl.id = $1
			AND cl.deleted_at IS NULL
	`
	rows, err := exec.Query(ctx, query, id, models.VALUES_COUNT_LIMIT+1)
	if err != nil {
		return models.CustomList{}, err
	}
	defer rows.Close()
	customsList, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.CustomList, error) {
		customList, err := adaptModelUsingRowToStruct(row, dbmodels.AdaptCustomList)
		if err != nil {
			return models.CustomList{}, err
		}
		return customList, nil
	})
	if err != nil {
		return models.CustomList{}, err
	}
	if len(customsList) == 0 {
		return models.CustomList{}, errors.Wrap(models.NotFoundError, "custom list not found")
	}
	return customsList[0], nil
}

func (repo *CustomListRepositoryPostgresql) GetCustomListValues(
	ctx context.Context,
	exec Executor,
	getCustomList models.GetCustomListValuesInput,
	forUpdate ...bool,
) ([]models.CustomListValue, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.ColumnsSelectCustomListValue...).
		From(dbmodels.TABLE_CUSTOM_LIST_VALUE).
		Where("custom_list_id = ? AND deleted_at IS NULL", getCustomList.Id)

	if len(forUpdate) > 0 && forUpdate[0] {
		query = query.Suffix("FOR UPDATE")
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptCustomListValue)
}

func (repo *CustomListRepositoryPostgresql) GetCustomListValueById(ctx context.Context, exec Executor, id string) (models.CustomListValue, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.CustomListValue{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectCustomListValue...).
			From(dbmodels.TABLE_CUSTOM_LIST_VALUE).
			Where("id = ? AND deleted_at IS NULL", id),
		dbmodels.AdaptCustomListValue,
	)
}

func (repo *CustomListRepositoryPostgresql) CreateCustomList(
	ctx context.Context,
	exec Executor,
	createCustomList models.CreateCustomListInput,
	newCustomListId string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_CUSTOM_LIST).
			Columns(
				"id",
				"organization_id",
				"name",
				"description",
			).
			Values(
				newCustomListId,
				createCustomList.OrganizationId,
				createCustomList.Name,
				createCustomList.Description,
			),
	)
	return err
}

func (repo *CustomListRepositoryPostgresql) UpdateCustomList(ctx context.Context, exec Executor, updateCustomList models.UpdateCustomListInput) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	updateRequest := NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST)

	if updateCustomList.Name != nil {
		updateRequest = updateRequest.Set("name", *updateCustomList.Name)
	}
	if updateCustomList.Description != nil {
		updateRequest = updateRequest.Set("description", *updateCustomList.Description)
	}
	updateRequest = updateRequest.Set("updated_at", squirrel.Expr("NOW()"))

	updateRequest = updateRequest.Where("id = ?", updateCustomList.Id)

	err := ExecBuilder(ctx, exec, updateRequest)
	return err
}

func (repo *CustomListRepositoryPostgresql) SoftDeleteCustomList(ctx context.Context, exec Executor, listId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	softDeleteRequest := NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST)
	softDeleteRequest = softDeleteRequest.Set("deleted_at", squirrel.Expr("NOW()"))
	softDeleteRequest = softDeleteRequest.Set("updated_at", squirrel.Expr("NOW()"))
	softDeleteRequest = softDeleteRequest.Where("id = ?", listId)

	err := ExecBuilder(ctx, exec, softDeleteRequest)
	return err
}

func (repo *CustomListRepositoryPostgresql) AddCustomListValue(
	ctx context.Context,
	exec Executor,
	addCustomListValue models.AddCustomListValueInput,
	newId string,
	userId *models.UserId,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	if userId != nil {
		if err := setCurrentUserIdContext(ctx, exec, userId); err != nil {
			return err
		}
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_CUSTOM_LIST_VALUE).
			Columns(
				"id",
				"custom_list_id",
				"value",
			).
			Values(
				newId,
				addCustomListValue.CustomListId,
				addCustomListValue.Value,
			),
	)
	return err
}

func (repo *CustomListRepositoryPostgresql) BatchInsertCustomListValues(
	ctx context.Context,
	exec Executor,
	customListId string,
	customListValues []models.BatchInsertCustomListValue,
	userId *models.UserId,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if len(customListValues) == 0 {
		return nil
	}

	if userId != nil {
		if err := setCurrentUserIdContext(ctx, exec, userId); err != nil {
			return err
		}
	}

	query := NewQueryBuilder().Insert(dbmodels.TABLE_CUSTOM_LIST_VALUE).
		Columns(
			"id",
			"custom_list_id",
			"value",
		)

	for _, addCustomListValue := range customListValues {
		query = query.Values(
			addCustomListValue.Id,
			customListId,
			addCustomListValue.Value,
		)
	}

	err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo *CustomListRepositoryPostgresql) DeleteCustomListValue(
	ctx context.Context,
	exec Executor,
	deleteCustomListValue models.DeleteCustomListValueInput,
	userId *models.UserId,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	if userId != nil {
		if err := setCurrentUserIdContext(ctx, exec, userId); err != nil {
			return err
		}
	}

	deleteRequest := NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST_VALUE)

	deleteRequest = deleteRequest.Set("deleted_at", squirrel.Expr("NOW()"))

	deleteRequest = deleteRequest.Where("id = ? AND custom_list_id = ?",
		deleteCustomListValue.Id, deleteCustomListValue.CustomListId)

	err := ExecBuilder(ctx, exec, deleteRequest)
	return err
}

func (repo *CustomListRepositoryPostgresql) BatchDeleteCustomListValues(
	ctx context.Context,
	exec Executor,
	customListId string,
	deleteCustomListValueIds []string,
	userId *models.UserId,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	if userId != nil {
		if err := setCurrentUserIdContext(ctx, exec, userId); err != nil {
			return err
		}
	}

	deleteRequest := NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST_VALUE)

	deleteRequest = deleteRequest.Set("deleted_at", squirrel.Expr("NOW()"))

	deleteRequest = deleteRequest.Where(map[string]interface{}{
		"custom_list_id": customListId,
		"id":             deleteCustomListValueIds,
	})

	err := ExecBuilder(ctx, exec, deleteRequest)
	return err
}
