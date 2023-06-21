package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ListRepository interface {
	AllLists(tx Transaction, orgId string) ([]models.List, error)
	GetListById(tx Transaction, getList models.GetListInput) (models.List, error)
	GetListValues(tx Transaction, getList models.GetListValuesInput) ([]models.ListValue, error)
	CreateList(tx Transaction, createList models.CreateListInput, newListId string) error
	UpdateList(tx Transaction, updateList models.UpdateListInput) error
	DeleteList(tx Transaction, deleteList models.DeleteListInput) error
	AddListValue(tx Transaction, addListValue models.AddListValueInput, newListId string) error
	DeleteListValue(tx Transaction, deleteListValue models.DeleteListValueInput) error
}

type ListRepositoryPostgresql struct {
	transactionFactory TransactionFactory
	queryBuilder       squirrel.StatementBuilderType
}

func (repo *ListRepositoryPostgresql) AllLists(tx Transaction, orgId string) ([]models.List, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		repo.queryBuilder.
			Select(dbmodels.ColumnsSelectList...).
			From(dbmodels.TABLE_LIST).
			Where("orgId = ?", orgId).
			OrderBy("id"),
		dbmodels.AdaptList,
	)
}
func (repo *ListRepositoryPostgresql) GetListById(tx Transaction, getList models.GetListInput) (models.List, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		repo.queryBuilder.
			Select(dbmodels.ColumnsSelectList...).
			From(dbmodels.TABLE_LIST).
			Where("id = ? AND orgId = ?", getList.Id, getList.OrgId),
		dbmodels.AdaptList,
	)
}


func (repo *ListRepositoryPostgresql) GetListValues(tx Transaction, getList models.GetListValuesInput) ([]models.ListValue, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		repo.queryBuilder.
			Select(dbmodels.ColumnsSelectListValue...).
			From(dbmodels.TABLE_LIST_VALUE).
			Join("? using (listId)", dbmodels.TABLE_LIST).
			Where("listId = ? AND orgId = ?", getList.Id, getList.OrgId),
		dbmodels.AdaptListValue,
	)
}

func (repo *ListRepositoryPostgresql) CreateList(tx Transaction, listId string, createList models.CreateListInput, newListId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		repo.queryBuilder.Insert(dbmodels.TABLE_LIST).
			Columns(
				"id",
				"orgId",
				"name",
				"description",
			).
			Values(
				newListId,
				createList.OrgId,
				createList.Name,
				createList.Description,
			),
	)
	return err
}

func (repo *ListRepositoryPostgresql) UpdateList(tx Transaction, updateList models.UpdateListInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	var updateRequest = repo.queryBuilder.Update(dbmodels.TABLE_LIST)

	if updateList.Name != nil {
		updateRequest = updateRequest.Set("name", *updateList.Name)
	}
	if updateList.Description != nil {
		updateRequest = updateRequest.Set("database_name", *updateList.Description)
	}
	updateRequest = updateRequest.Set("updated_at", squirrel.Expr("NOW()"))

	updateRequest = updateRequest.Where("id = ? AND orgId = ?", updateList.Id, updateList.OrgId)

	_, err := pgTx.ExecBuilder(updateRequest)
	return err
}

func (repo *ListRepositoryPostgresql) DeleteList(tx Transaction, deleteList models.DeleteListInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(repo.queryBuilder.Delete(dbmodels.TABLE_LIST).Where("id = ? AND orgId = ?", deleteList.Id, deleteList.OrgId))
	return err
}

func (repo *ListRepositoryPostgresql) AddListValue(tx Transaction, addListValue models.AddListValueInput, newListId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		repo.queryBuilder.Insert(dbmodels.TABLE_LIST_VALUE).
			Columns(
				"id",
				"listId",
				"value",
			).
			Values(
				newListId,
				addListValue.ListId,
				addListValue.Value,
			),
	)
	return err
}


func (repo *ListRepositoryPostgresql) DeleteListValue(tx Transaction, deleteListValue models.DeleteListValueInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	var updateRequest = repo.queryBuilder.Update(dbmodels.TABLE_LIST_VALUE)

	updateRequest = updateRequest.Set("deleted_at", squirrel.Expr("NOW()"))

	updateRequest = updateRequest.Where("id = ? AND list_id = ?", deleteListValue.Id, deleteListValue.ListId)

	_, err := pgTx.ExecBuilder(updateRequest)
	return err
}