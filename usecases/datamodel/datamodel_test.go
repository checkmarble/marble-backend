package datamodel

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

func TestUseCase_CreateTable(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		tableName := "name"
		defaultFields := []models.DataModelField{
			{
				Name:        "object_id",
				Description: fmt.Sprintf("required id on all objects in the %s table", tableName),
				Type:        models.String.String(),
			},
			{
				Name:        "updated_at",
				Description: fmt.Sprintf("required timestamp on all objects in the %s table", tableName),
				Type:        models.Timestamp.String(),
			},
		}

		tableID := uuid.NewString()

		mockRepository := new(mocks.Database)
		mockRepository.On("CreateDataModelTable", mock.Anything, "organization", tableName, "description", defaultFields).
			Return(tableID, nil)

		useCase := UseCase{
			repository: mockRepository,
		}

		got, err := useCase.CreateTable(context.Background(), "organization", tableName, "description")
		assert.NoError(t, err)
		assert.Equal(t, tableID, got)
		mockRepository.AssertExpectations(t)
	})

	t.Run("CreateDataModelTable error", func(t *testing.T) {
		tableName := "name"
		defaultFields := []models.DataModelField{
			{
				Name:        "object_id",
				Description: fmt.Sprintf("required id on all objects in the %s table", tableName),
				Type:        models.String.String(),
			},
			{
				Name:        "updated_at",
				Description: fmt.Sprintf("required timestamp on all objects in the %s table", tableName),
				Type:        models.Timestamp.String(),
			},
		}

		mockRepository := new(mocks.Database)
		mockRepository.On("CreateDataModelTable", mock.Anything, "organization", tableName, "description", defaultFields).
			Return("", assert.AnError)

		useCase := UseCase{
			repository: mockRepository,
		}

		_, err := useCase.CreateTable(context.Background(), "organization", tableName, "description")
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
	})
}

func TestUseCase_CreateField(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		tableID := uuid.NewString()
		fieldID := uuid.NewString()
		field := models.DataModelField{
			Name:        "name",
			Description: "description",
			Type:        models.Int.String(),
			Nullable:    true,
			IsEnum:      true,
		}

		mockRepository := new(mocks.Database)
		mockRepository.On("CreateDataModelField", mock.Anything, "organization", tableID, field).
			Return(fieldID, nil)

		useCase := UseCase{
			repository: mockRepository,
		}

		got, err := useCase.CreateField(context.Background(), "organization", tableID, field)
		assert.NoError(t, err)
		assert.Equal(t, fieldID, got)
		mockRepository.AssertExpectations(t)
	})

	t.Run("CreateDataModelField error", func(t *testing.T) {
		tableID := uuid.NewString()
		field := models.DataModelField{
			Name:        "name",
			Description: "description",
			Type:        models.Int.String(),
			Nullable:    true,
			IsEnum:      true,
		}

		mockRepository := new(mocks.Database)
		mockRepository.On("CreateDataModelField", mock.Anything, "organization", tableID, field).
			Return("", assert.AnError)

		useCase := UseCase{
			repository: mockRepository,
		}

		_, err := useCase.CreateField(context.Background(), "organization", tableID, field)
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
	})
}

func TestUseCase_GetDataModel(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		dataModel := models.DataModel{
			Version: "version",
			Status:  models.Live,
			Tables: map[models.TableName]models.Table{
				"accounts": {
					ID:          uuid.NewString(),
					Name:        "accounts",
					Description: "description",
					Fields: map[models.FieldName]models.Field{
						"id": {
							ID:          uuid.NewString(),
							Description: "description",
							DataType:    models.Int,
							Nullable:    false,
							IsEnum:      false,
						},
					},
				},
				"transactions": {
					ID:          uuid.NewString(),
					Name:        "transactions",
					Description: "description",
					Fields: map[models.FieldName]models.Field{
						"id": {
							ID:          uuid.NewString(),
							Description: "description",
							DataType:    models.Int,
							Nullable:    false,
							IsEnum:      false,
						},
					},
				},
			},
		}

		mockRepository := new(mocks.Database)
		mockRepository.On("GetDataModel", mock.Anything, "organization", true).
			Return(dataModel, nil)

		useCase := UseCase{
			repository: mockRepository,
		}

		got, err := useCase.GetDataModel(context.Background(), "organization")
		assert.NoError(t, err)
		assert.Equal(t, dataModel, got)
		mockRepository.AssertExpectations(t)
	})

	t.Run("GetDataModel error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetDataModel", mock.Anything, "organization", true).
			Return(models.DataModel{}, assert.AnError)

		useCase := UseCase{
			repository: mockRepository,
		}

		_, err := useCase.GetDataModel(context.Background(), "organization")
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
	})
}

func TestUseCase_CreateDataModelLink(t *testing.T) {
	link := models.DataModelLink{
		OrganizationID: uuid.NewString(),
		Name:           "name",
		ParentTableID:  uuid.NewString(),
		ParentFieldID:  uuid.NewString(),
		ChildTableID:   uuid.NewString(),
		ChildFieldID:   uuid.NewString(),
	}

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("CreateDataModelLink", mock.Anything, link).
			Return(nil)

		useCase := UseCase{
			repository: mockRepository,
		}

		err := useCase.CreateDataModelLink(context.Background(), link)
		assert.NoError(t, err)
		mockRepository.AssertExpectations(t)
	})

	t.Run("CreateDataModelLink error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("CreateDataModelLink", mock.Anything, link).
			Return(assert.AnError)

		useCase := UseCase{
			repository: mockRepository,
		}

		err := useCase.CreateDataModelLink(context.Background(), link)
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
	})
}

func TestUseCase_UpdateDataModelTable(t *testing.T) {
	tableID := uuid.NewString()
	description := "new description"

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("UpdateDataModelTable", mock.Anything, tableID, description).
			Return(nil)

		useCase := UseCase{
			repository: mockRepository,
		}

		err := useCase.UpdateDataModelTable(context.Background(), tableID, description)
		assert.NoError(t, err)
		mockRepository.AssertExpectations(t)
	})

	t.Run("UpdateDataModelTable error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("UpdateDataModelTable", mock.Anything, tableID, description).
			Return(assert.AnError)

		useCase := UseCase{
			repository: mockRepository,
		}

		err := useCase.UpdateDataModelTable(context.Background(), tableID, description)
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
	})
}

func TestUseCase_UpdateDataModelField(t *testing.T) {
	fieldID := uuid.NewString()
	description := "new description"

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetDataModelField", mock.Anything, fieldID).Return(models.Field{}, nil)
		mockRepository.On("UpdateDataModelField", mock.Anything, fieldID, models.UpdateDataModelFieldInput{
			Description: &description, IsEnum: nil,
		}).
			Return(nil)

		useCase := UseCase{
			repository: mockRepository,
		}

		err := useCase.UpdateDataModelField(context.Background(), fieldID, models.UpdateDataModelFieldInput{
			Description: &description, IsEnum: nil,
		})
		assert.NoError(t, err)
		mockRepository.AssertExpectations(t)
	})

	t.Run("Error when setting isEnum on a bool field", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetDataModelField", mock.Anything, fieldID).Return(models.Field{
			DataType: models.Bool,
		}, nil)

		useCase := UseCase{
			repository: mockRepository,
		}

		trueVar := true
		err := useCase.UpdateDataModelField(context.Background(), fieldID, models.UpdateDataModelFieldInput{
			Description: &description, IsEnum: &trueVar,
		})
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
	})

	t.Run("UpdateDataModelField error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("GetDataModelField", mock.Anything, fieldID).Return(models.Field{}, nil)
		mockRepository.On("UpdateDataModelField", mock.Anything, fieldID, models.UpdateDataModelFieldInput{
			Description: &description, IsEnum: nil,
		}).
			Return(assert.AnError)

		useCase := UseCase{
			repository: mockRepository,
		}

		err := useCase.UpdateDataModelField(context.Background(), fieldID, models.UpdateDataModelFieldInput{
			Description: &description, IsEnum: nil,
		})
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
	})
}

func TestUseCase_DeleteDataModel(t *testing.T) {
	organizationID := uuid.NewString()

	t.Run("nominal", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("DeleteDataModel", mock.Anything, organizationID).
			Return(nil)

		useCase := UseCase{
			repository: mockRepository,
		}

		err := useCase.DeleteDataModel(context.Background(), organizationID)
		assert.NoError(t, err)
		mockRepository.AssertExpectations(t)
	})

	t.Run("DeleteDataModel error", func(t *testing.T) {
		mockRepository := new(mocks.Database)
		mockRepository.On("DeleteDataModel", mock.Anything, organizationID).
			Return(assert.AnError)

		useCase := UseCase{
			repository: mockRepository,
		}

		err := useCase.DeleteDataModel(context.Background(), organizationID)
		assert.Error(t, err)
		mockRepository.AssertExpectations(t)
	})
}
