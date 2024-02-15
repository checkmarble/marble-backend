package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func TestDataModelHandler_GetDataModel(t *testing.T) {
	organizationID := uuid.NewString()
	credentials := models.Credentials{
		OrganizationId: organizationID,
		Role:           models.ADMIN,
	}
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
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)

	t.Run("nominal", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("GetDataModel", mock.Anything, organizationID).
			Return(dataModel, nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/datamodel", dataModelHandler.GetDataModel)
		request := httptest.NewRequest(http.MethodGet, "https://checkmarble.com/datamodel", nil).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		data := map[string]interface{}{
			"data_model": dto.AdaptDataModelDto(dataModel),
		}
		expected, _ := json.Marshal(data)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, string(expected), r.Body.String())
		mockUseCase.AssertExpectations(t)
	})

	t.Run("GetDataModel error", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("GetDataModel", mock.Anything, organizationID).
			Return(models.DataModel{}, assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/datamodel", dataModelHandler.GetDataModel)
		request := httptest.NewRequest(http.MethodGet, "https://checkmarble.com/datamodel", nil).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusInternalServerError, r.Code)
		mockUseCase.AssertExpectations(t)
	})
}

func TestDataModelHandler_CreateTable(t *testing.T) {
	tableID := uuid.NewString()
	organizationID := uuid.NewString()
	credentials := models.Credentials{
		OrganizationId: organizationID,
		Role:           models.ADMIN,
	}
	body := `{"name": "name", "description": "description"}`
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)

	t.Run("nominal", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateTable", mock.Anything, organizationID, "name", "description").
			Return(tableID, nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/datamodel/tables", dataModelHandler.CreateTable)
		request := httptest.NewRequest(http.MethodPost,
			"https://checkmarble.com/datamodel/tables", strings.NewReader(body)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		expected := fmt.Sprintf(`{"id": "%s"}`, tableID)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, expected, r.Body.String())
		mockUseCase.AssertExpectations(t)
	})

	t.Run("CreateTable error", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateTable", mock.Anything, organizationID, "name", "description").
			Return("", assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/datamodel/tables", dataModelHandler.CreateTable)
		request := httptest.NewRequest(http.MethodPost,
			"https://checkmarble.com/datamodel/tables", strings.NewReader(body)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusInternalServerError, r.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("bad data", func(t *testing.T) {
		dataModelHandler := DataModelHandler{}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/datamodel/tables", dataModelHandler.CreateTable)
		request := httptest.NewRequest(http.MethodPost,
			"https://checkmarble.com/datamodel/tables", strings.NewReader(`{bad}`)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusBadRequest, r.Code)
	})
}

func TestDataModelHandler_CreateField(t *testing.T) {
	tableID := uuid.NewString()
	fieldID := uuid.NewString()
	organizationID := uuid.NewString()
	credentials := models.Credentials{
		OrganizationId: organizationID,
		Role:           models.ADMIN,
	}
	field := models.DataModelField{
		Name:        "name",
		Description: "description",
		Type:        models.Int.String(),
		Nullable:    true,
		IsEnum:      true,
	}
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)

	t.Run("nominal", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateField", mock.Anything, organizationID, tableID, field).
			Return(fieldID, nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/tables/:tableID/fields", dataModelHandler.CreateField)

		body := strings.NewReader(`{"name": "name", "description": "description", "type": "Int", "nullable": true, "is_enum": true}`)
		endpoint := fmt.Sprintf("https://checkmarble.com/tables/%s/fields", tableID)
		request := httptest.NewRequest(http.MethodPost, endpoint, body).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		expected := fmt.Sprintf(`{"id": "%s"}`, fieldID)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, expected, r.Body.String())
		mockUseCase.AssertExpectations(t)
	})

	t.Run("CreateField error", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateField", mock.Anything, organizationID, tableID, field).
			Return("", assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/tables/:tableID/fields", dataModelHandler.CreateField)

		body := strings.NewReader(`{"name": "name", "description": "description", "type": "Int", "nullable": true, "is_enum": true}`)
		endpoint := fmt.Sprintf("https://checkmarble.com/tables/%s/fields", tableID)

		request := httptest.NewRequest(http.MethodPost, endpoint, body).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusInternalServerError, r.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("bad body", func(t *testing.T) {
		dataModelHandler := DataModelHandler{}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/tables/:tableID/fields", dataModelHandler.CreateField)

		body := strings.NewReader(`{bad}`)
		endpoint := fmt.Sprintf("https://checkmarble.com/tables/%s/fields", tableID)

		request := httptest.NewRequest(http.MethodPost, endpoint, body).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusBadRequest, r.Code)
	})
}

func TestDataModelHandler_CreateLink(t *testing.T) {
	organizationID := uuid.NewString()
	credentials := models.Credentials{
		OrganizationId: organizationID,
		Role:           models.ADMIN,
	}
	link := models.DataModelLink{
		OrganizationID: organizationID,
		Name:           "name",
		ParentTableID:  uuid.NewString(),
		ParentFieldID:  uuid.NewString(),
		ChildTableID:   uuid.NewString(),
		ChildFieldID:   uuid.NewString(),
	}

	body := fmt.Sprintf(`{"name": "name", "parent_table_id": "%s", "parent_field_id": "%s", "child_table_id": "%s", "child_field_id": "%s"}`,
		link.ParentTableID,
		link.ParentFieldID,
		link.ChildTableID,
		link.ChildFieldID,
	)

	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)

	t.Run("nominal", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateDataModelLink", mock.Anything, link).
			Return(nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/datamodel/links", dataModelHandler.CreateLink)

		request := httptest.NewRequest(http.MethodPost,
			"https://checkmarble.com/datamodel/links", strings.NewReader(body)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusNoContent, r.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("CreateDataModelLink error", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateDataModelLink", mock.Anything, link).
			Return(assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/datamodel/links", dataModelHandler.CreateLink)

		request := httptest.NewRequest(http.MethodPost,
			"https://checkmarble.com/datamodel/links", strings.NewReader(body)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusInternalServerError, r.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("bad body", func(t *testing.T) {
		dataModelHandler := DataModelHandler{}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/datamodel/links", dataModelHandler.CreateLink)

		request := httptest.NewRequest(http.MethodPost,
			"https://checkmarble.com/datamodel/links", strings.NewReader(`{bad}`)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusBadRequest, r.Code)
	})
}

func TestDataModelHandler_UpdateDataModelTable(t *testing.T) {
	organizationID := uuid.NewString()
	credentials := models.Credentials{
		OrganizationId: organizationID,
		Role:           models.ADMIN,
	}
	tableID := uuid.NewString()
	description := "new description"
	body := fmt.Sprintf(`{"description": "%s"}`, description)
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)

	t.Run("nominal", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("UpdateDataModelTable", mock.Anything, tableID, description).
			Return(nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PATCH("/datamodel/tables/:tableID", dataModelHandler.UpdateDataModelTable)

		endpoint := fmt.Sprintf("https://checkmarble.com/datamodel/tables/%s", tableID)
		request := httptest.NewRequest(http.MethodPatch, endpoint, strings.NewReader(body)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusNoContent, r.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("UpdateDataModelTable error", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("UpdateDataModelTable", mock.Anything, tableID, description).
			Return(assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PATCH("/datamodel/tables/:tableID", dataModelHandler.UpdateDataModelTable)

		endpoint := fmt.Sprintf("https://checkmarble.com/datamodel/tables/%s", tableID)
		request := httptest.NewRequest(http.MethodPatch, endpoint, strings.NewReader(body)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusInternalServerError, r.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("bad body", func(t *testing.T) {
		dataModelHandler := DataModelHandler{}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PATCH("/datamodel/tables/:tableID", dataModelHandler.UpdateDataModelTable)

		endpoint := fmt.Sprintf("https://checkmarble.com/datamodel/tables/%s", tableID)
		request := httptest.NewRequest(http.MethodPatch, endpoint, strings.NewReader(`{bad}`)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusBadRequest, r.Code)
	})
}

func TestDataModelHandler_UpdateDataModelField(t *testing.T) {
	organizationID := uuid.NewString()
	credentials := models.Credentials{
		OrganizationId: organizationID,
		Role:           models.ADMIN,
	}
	fieldID := uuid.NewString()
	description := "new description"
	body := fmt.Sprintf(`{"description": "%s"}`, description)
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)

	t.Run("nominal", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("UpdateDataModelField", mock.Anything, fieldID, models.UpdateDataModelFieldInput{
			Description: &description, IsEnum: nil,
		}).
			Return(nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PATCH("/datamodel/fields/:fieldID", dataModelHandler.UpdateDataModelField)

		endpoint := fmt.Sprintf("https://checkmarble.com/datamodel/fields/%s", fieldID)
		request := httptest.NewRequest(http.MethodPatch, endpoint, strings.NewReader(body)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusNoContent, r.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("UpdateDataModelField error", func(t *testing.T) {
		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("UpdateDataModelField", mock.Anything, fieldID, models.UpdateDataModelFieldInput{
			Description: &description, IsEnum: nil,
		}).
			Return(assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PATCH("/datamodel/fields/:fieldID", dataModelHandler.UpdateDataModelField)

		endpoint := fmt.Sprintf("https://checkmarble.com/datamodel/fields/%s", fieldID)
		request := httptest.NewRequest(http.MethodPatch, endpoint, strings.NewReader(body)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusInternalServerError, r.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("bad body", func(t *testing.T) {
		dataModelHandler := DataModelHandler{}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PATCH("/datamodel/fields/:fieldID", dataModelHandler.UpdateDataModelField)

		endpoint := fmt.Sprintf("https://checkmarble.com/datamodel/fields/%s", fieldID)
		request := httptest.NewRequest(http.MethodPatch, endpoint, strings.NewReader(`{bad}`)).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, request)

		assert.Equal(t, http.StatusBadRequest, r.Code)
	})
}
