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

func (cl *CustomListRepository) AllCustomLists(ctx context.Context, tx repositories.Transaction, organizationId string) ([]models.CustomList, error) {
	args := cl.Called(tx, organizationId)
	return args.Get(0).([]models.CustomList), args.Error(1)
}

func (cl *CustomListRepository) GetCustomListById(ctx context.Context, tx repositories.Transaction, id string) (models.CustomList, error) {
	args := cl.Called(tx, id)
	return args.Get(0).(models.CustomList), args.Error(1)
}

func (cl *CustomListRepository) GetCustomListValues(ctx context.Context, tx repositories.Transaction, getCustomList models.GetCustomListValuesInput) ([]models.CustomListValue, error) {
	args := cl.Called(tx, getCustomList)
	return args.Get(0).([]models.CustomListValue), args.Error(1)
}

func (cl *CustomListRepository) GetCustomListValueById(ctx context.Context, tx repositories.Transaction, id string) (models.CustomListValue, error) {
	args := cl.Called(tx, id)
	return args.Get(0).(models.CustomListValue), args.Error(1)
}

func (cl *CustomListRepository) CreateCustomList(ctx context.Context, tx repositories.Transaction, createCustomList models.CreateCustomListInput, organizationId string, newCustomListId string) error {
	args := cl.Called(tx, createCustomList)
	return args.Error(0)
}

func (cl *CustomListRepository) UpdateCustomList(ctx context.Context, tx repositories.Transaction, updateCustomList models.UpdateCustomListInput) error {
	args := cl.Called(tx, updateCustomList)
	return args.Error(0)
}

func (cl *CustomListRepository) SoftDeleteCustomList(ctx context.Context, tx repositories.Transaction, listId string) error {
	args := cl.Called(tx, listId)
	return args.Error(0)
}

func (cl *CustomListRepository) AddCustomListValue(ctx context.Context, tx repositories.Transaction, addCustomListValue models.AddCustomListValueInput, newCustomListId string) error {
	args := cl.Called(tx, addCustomListValue)
	return args.Error(0)
}

func (cl *CustomListRepository) DeleteCustomListValue(ctx context.Context, tx repositories.Transaction, deleteCustomListValue models.DeleteCustomListValueInput) error {
	args := cl.Called(tx, deleteCustomListValue)
	return args.Error(0)
}
