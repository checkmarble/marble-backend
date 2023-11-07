package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type CustomListRepository struct {
	mock.Mock
}

func (cl *CustomListRepository) AllCustomLists(tx repositories.Transaction, organizationId string) ([]models.CustomList, error) {
	args := cl.Called(tx, organizationId)
	return args.Get(0).([]models.CustomList), args.Error(1)
}

func (cl *CustomListRepository) GetCustomListById(tx repositories.Transaction, id string) (models.CustomList, error) {
	args := cl.Called(tx, id)
	return args.Get(0).(models.CustomList), args.Error(1)
}

func (cl *CustomListRepository) GetCustomListValues(tx repositories.Transaction, getCustomList models.GetCustomListValuesInput) ([]models.CustomListValue, error) {
	args := cl.Called(tx, getCustomList)
	return args.Get(0).([]models.CustomListValue), args.Error(1)
}

func (cl *CustomListRepository) GetCustomListValueById(tx repositories.Transaction, id string) (models.CustomListValue, error) {
	args := cl.Called(tx, id)
	return args.Get(0).(models.CustomListValue), args.Error(1)
}

func (cl *CustomListRepository) CreateCustomList(tx repositories.Transaction, createCustomList models.CreateCustomListInput, organizationId string, newCustomListId string) error {
	args := cl.Called(tx, createCustomList)
	return args.Error(0)
}

func (cl *CustomListRepository) UpdateCustomList(tx repositories.Transaction, updateCustomList models.UpdateCustomListInput) error {
	args := cl.Called(tx, updateCustomList)
	return args.Error(0)
}

func (cl *CustomListRepository) SoftDeleteCustomList(tx repositories.Transaction, listId string) error {
	args := cl.Called(tx, listId)
	return args.Error(0)
}

func (cl *CustomListRepository) AddCustomListValue(tx repositories.Transaction, addCustomListValue models.AddCustomListValueInput, newCustomListId string) error {
	args := cl.Called(tx, addCustomListValue)
	return args.Error(0)
}

func (cl *CustomListRepository) DeleteCustomListValue(tx repositories.Transaction, deleteCustomListValue models.DeleteCustomListValueInput) error {
	args := cl.Called(tx, deleteCustomListValue)
	return args.Error(0)
}
