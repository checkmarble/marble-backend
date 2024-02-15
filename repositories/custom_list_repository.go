package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type CustomListRepository interface {
	AllCustomLists(ctx context.Context, exec Executor, organizationId string) ([]models.CustomList, error)
	GetCustomListById(ctx context.Context, exec Executor, id string) (models.CustomList, error)
	GetCustomListValues(ctx context.Context, exec Executor, getCustomList models.GetCustomListValuesInput) ([]models.CustomListValue, error)
	GetCustomListValueById(ctx context.Context, exec Executor, id string) (models.CustomListValue, error)
	CreateCustomList(ctx context.Context, exec Executor, createCustomList models.CreateCustomListInput, organizationId string, newCustomListId string) error
	UpdateCustomList(ctx context.Context, exec Executor, updateCustomList models.UpdateCustomListInput) error
	SoftDeleteCustomList(ctx context.Context, exec Executor, listId string) error
	AddCustomListValue(ctx context.Context, exec Executor, addCustomListValue models.AddCustomListValueInput, newCustomListId string) error
	DeleteCustomListValue(ctx context.Context, exec Executor, deleteCustomListValue models.DeleteCustomListValueInput) error
}

type CustomListRepositoryPostgresql struct {
	executorGetter ExecutorGetter
}

func (repo *CustomListRepositoryPostgresql) AllCustomLists(ctx context.Context, exec Executor, organizationId string) ([]models.CustomList, error) {
	exec = repo.executorGetter.ifNil(exec)

	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectCustomList...).
			From(dbmodels.TABLE_CUSTOM_LIST).
			Where("organization_id = ? AND deleted_at IS NULL", organizationId).
			OrderBy("id"),
		dbmodels.AdaptCustomList,
	)
}
func (repo *CustomListRepositoryPostgresql) GetCustomListById(ctx context.Context, exec Executor, id string) (models.CustomList, error) {
	exec = repo.executorGetter.ifNil(exec)

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectCustomList...).
			From(dbmodels.TABLE_CUSTOM_LIST).
			Where("id = ? AND deleted_at IS NULL", id),
		dbmodels.AdaptCustomList,
	)
}

func (repo *CustomListRepositoryPostgresql) GetCustomListValues(ctx context.Context, exec Executor, getCustomList models.GetCustomListValuesInput) ([]models.CustomListValue, error) {
	exec = repo.executorGetter.ifNil(exec)

	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectCustomListValue...).
			From(dbmodels.TABLE_CUSTOM_LIST_VALUE).
			Where("custom_list_id = ? AND deleted_at IS NULL", getCustomList.Id),
		dbmodels.AdaptCustomListValue,
	)
}
func (repo *CustomListRepositoryPostgresql) GetCustomListValueById(ctx context.Context, exec Executor, id string) (models.CustomListValue, error) {
	exec = repo.executorGetter.ifNil(exec)

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
	organizationId string,
	newCustomListId string,
) error {
	exec = repo.executorGetter.ifNil(exec)

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
				organizationId,
				createCustomList.Name,
				createCustomList.Description,
			),
	)
	return err
}

func (repo *CustomListRepositoryPostgresql) UpdateCustomList(ctx context.Context, exec Executor, updateCustomList models.UpdateCustomListInput) error {
	exec = repo.executorGetter.ifNil(exec)

	var updateRequest = NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST)

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
	exec = repo.executorGetter.ifNil(exec)
	var softDeleteRequest = NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST)
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
	newCustomListId string,
) error {
	exec = repo.executorGetter.ifNil(exec)

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
				newCustomListId,
				addCustomListValue.CustomListId,
				addCustomListValue.Value,
			),
	)
	return err
}

func (repo *CustomListRepositoryPostgresql) DeleteCustomListValue(
	ctx context.Context,
	exec Executor,
	deleteCustomListValue models.DeleteCustomListValueInput,
) error {
	exec = repo.executorGetter.ifNil(exec)

	var deleteRequest = NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST_VALUE)

	deleteRequest = deleteRequest.Set("deleted_at", squirrel.Expr("NOW()"))

	deleteRequest = deleteRequest.Where("id = ? AND custom_list_id = ?", deleteCustomListValue.Id, deleteCustomListValue.CustomListId)

	err := ExecBuilder(ctx, exec, deleteRequest)
	return err
}
