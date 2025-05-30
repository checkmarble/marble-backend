package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CustomListRepository struct {
	mock.Mock
}

func (cl *CustomListRepository) AllCustomLists(ctx context.Context, exec repositories.Executor, organizationId string) ([]models.CustomList, error) {
	args := cl.Called(exec, organizationId)
	return args.Get(0).([]models.CustomList), args.Error(1)
}

func (cl *CustomListRepository) GetCustomListById(ctx context.Context, exec repositories.Executor,
	id string, includeDeleted bool,
) (models.CustomList, error) {
	args := cl.Called(exec, id, includeDeleted)
	return args.Get(0).(models.CustomList), args.Error(1)
}

func (cl *CustomListRepository) GetCustomListValues(ctx context.Context, exec repositories.Executor,
	getCustomList models.GetCustomListValuesInput, forUpdate ...bool,
) ([]models.CustomListValue, error) {
	args := cl.Called(exec, getCustomList)
	return args.Get(0).([]models.CustomListValue), args.Error(1)
}

func (cl *CustomListRepository) GetCustomListValueById(ctx context.Context,
	exec repositories.Executor, id string,
) (models.CustomListValue, error) {
	args := cl.Called(exec, id)
	return args.Get(0).(models.CustomListValue), args.Error(1)
}

func (cl *CustomListRepository) CreateCustomList(ctx context.Context, exec repositories.Executor,
	createCustomList models.CreateCustomListInput, newCustomListId string,
) error {
	args := cl.Called(ctx, exec, createCustomList, newCustomListId)
	return args.Error(0)
}

func (cl *CustomListRepository) UpdateCustomList(ctx context.Context, exec repositories.Executor, updateCustomList models.UpdateCustomListInput) error {
	args := cl.Called(exec, updateCustomList)
	return args.Error(0)
}

func (cl *CustomListRepository) SoftDeleteCustomList(ctx context.Context, exec repositories.Executor, listId string) error {
	args := cl.Called(exec, listId)
	return args.Error(0)
}

func (cl *CustomListRepository) AddCustomListValue(
	ctx context.Context,
	exec repositories.Executor,
	addCustomListValue models.AddCustomListValueInput,
	newCustomListId string,
	userId *models.UserId,
) error {
	args := cl.Called(ctx, exec, addCustomListValue, userId)
	return args.Error(0)
}

func (cl *CustomListRepository) BatchInsertCustomListValues(
	ctx context.Context,
	exec repositories.Executor,
	customListId string,
	customListValues []models.BatchInsertCustomListValue,
	userId *models.UserId,
) error {
	args := cl.Called(ctx, exec, customListId, customListValues, userId)
	return args.Error(0)
}

func (cl *CustomListRepository) DeleteCustomListValue(
	ctx context.Context,
	exec repositories.Executor,
	deleteCustomListValue models.DeleteCustomListValueInput,
	userId *models.UserId,
) error {
	args := cl.Called(ctx, exec, deleteCustomListValue, userId)
	return args.Error(0)
}

func (cl *CustomListRepository) BatchDeleteCustomListValues(
	ctx context.Context,
	exec repositories.Executor,
	customListId string,
	deleteCustomListValueIds []string,
	userId *models.UserId,
) error {
	args := cl.Called(ctx, exec, customListId, deleteCustomListValueIds, userId)
	return args.Error(0)
}
