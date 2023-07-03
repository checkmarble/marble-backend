package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type CustomListRepository interface {
	AllCustomLists(tx Transaction, orgId string) ([]models.CustomList, error)
	GetCustomListById(tx Transaction, id string) (models.CustomList, error)
	GetCustomListValues(tx Transaction, getCustomList models.GetCustomListValuesInput) ([]models.CustomListValue, error)
	CreateCustomList(tx Transaction, createCustomList models.CreateCustomListInput, newCustomListId string) error	
	UpdateCustomList(tx Transaction, updateCustomList models.UpdateCustomListInput) error
	SoftDeleteCustomList(tx Transaction, deleteCustomList models.DeleteCustomListInput) error
	AddCustomListValue(tx Transaction, addCustomListValue models.AddCustomListValueInput, newCustomListId string) error
	DeleteCustomListValue(tx Transaction, deleteCustomListValue models.DeleteCustomListValueInput) error
}

type CustomListRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func (repo *CustomListRepositoryPostgresql) AllCustomLists(tx Transaction, orgId string) ([]models.CustomList, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectCustomList...).
			From(dbmodels.TABLE_CUSTOM_LIST).
			Where("organization_id = ? AND deleted_at IS NULL", orgId).
			OrderBy("id"),
		dbmodels.AdaptCustomList,
	)
}
func (repo *CustomListRepositoryPostgresql) GetCustomListById(tx Transaction, id string) (models.CustomList, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectCustomList...).
			From(dbmodels.TABLE_CUSTOM_LIST).
			Where("id = ?", id),
		dbmodels.AdaptCustomList,
	)
}


func (repo *CustomListRepositoryPostgresql) GetCustomListValues(tx Transaction, getCustomList models.GetCustomListValuesInput) ([]models.CustomListValue, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectCustomListValue...).
			From(dbmodels.TABLE_CUSTOM_LIST_VALUE).
			Where("custom_list_id = ? AND deleted_at IS NULL", getCustomList.Id),
		dbmodels.AdaptCustomListValue,
	)
}

func (repo *CustomListRepositoryPostgresql) CreateCustomList(tx Transaction, createCustomList models.CreateCustomListInput, newCustomListId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_CUSTOM_LIST).
			Columns(
				"id",
				"organization_id",
				"name",
				"description",
			).
			Values(
				newCustomListId,
				createCustomList.OrgId,
				createCustomList.Name,
				createCustomList.Description,
			),
	)
	return err
}

func (repo *CustomListRepositoryPostgresql) UpdateCustomList(tx Transaction, updateCustomList models.UpdateCustomListInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	var updateRequest = NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST)

	if updateCustomList.Name != nil {
		updateRequest = updateRequest.Set("name", *updateCustomList.Name)
	}
	if updateCustomList.Description != nil {
		updateRequest = updateRequest.Set("description", *updateCustomList.Description)
	}
	updateRequest = updateRequest.Set("updated_at", squirrel.Expr("NOW()"))

	updateRequest = updateRequest.Where("id = ? AND organization_id = ?", updateCustomList.Id, updateCustomList.OrgId)

	_, err := pgTx.ExecBuilder(updateRequest)
	return err
}

func (repo *CustomListRepositoryPostgresql) SoftDeleteCustomList(tx Transaction, deleteCustomList models.DeleteCustomListInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	var softDeleteRequest = NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST)
	softDeleteRequest = softDeleteRequest.Set("deleted_at", squirrel.Expr("NOW()"))
	softDeleteRequest = softDeleteRequest.Set("updated_at", squirrel.Expr("NOW()"))
	softDeleteRequest = softDeleteRequest.Where("id = ? AND organization_id = ?", deleteCustomList.Id, deleteCustomList.OrgId)

	_, err := pgTx.ExecBuilder(softDeleteRequest)
	return err
}

func (repo *CustomListRepositoryPostgresql) AddCustomListValue(tx Transaction, addCustomListValue models.AddCustomListValueInput, newCustomListId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
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


func (repo *CustomListRepositoryPostgresql) DeleteCustomListValue(tx Transaction, deleteCustomListValue models.DeleteCustomListValueInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	var deleteRequest = NewQueryBuilder().Update(dbmodels.TABLE_CUSTOM_LIST_VALUE)

	deleteRequest = deleteRequest.Set("deleted_at", squirrel.Expr("NOW()"))

	deleteRequest = deleteRequest.Where("id = ? AND custom_list_id = ?", deleteCustomListValue.Id, deleteCustomListValue.CustomListId)

	_, err := pgTx.ExecBuilder(deleteRequest)
	return err
}