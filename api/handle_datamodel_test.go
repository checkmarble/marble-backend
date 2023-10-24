package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
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

	t.Run("nominal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", nil).
			WithContext(context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials))

		res := httptest.NewRecorder()

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("GetDataModel", mock.Anything, organizationID).
			Return(dataModel, nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.GetDataModel)
		handler.ServeHTTP(res, req)

		data := map[string]interface{}{
			"data_model": dto.AdaptDataModelDto(dataModel),
		}
		expected, _ := json.Marshal(data)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.JSONEq(t, string(expected), res.Body.String())
		mockUseCase.AssertExpectations(t)
	})

	t.Run("GetDataModel error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", nil).
			WithContext(context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials))

		res := httptest.NewRecorder()

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("GetDataModel", mock.Anything, organizationID).
			Return(models.DataModel{}, assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.GetDataModel)
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusInternalServerError, res.Code)
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

	t.Run("nominal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", strings.NewReader(body)).
			WithContext(context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials))

		res := httptest.NewRecorder()

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateTable", mock.Anything, organizationID, "name", "description").
			Return(tableID, nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.CreateTable)
		handler.ServeHTTP(res, req)

		expected := fmt.Sprintf(`{"id": "%s"}`, tableID)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.JSONEq(t, expected, res.Body.String())
		mockUseCase.AssertExpectations(t)
	})

	t.Run("CreateTable error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", strings.NewReader(body)).
			WithContext(context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials))

		res := httptest.NewRecorder()

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateTable", mock.Anything, organizationID, "name", "description").
			Return("", assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.CreateTable)
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusInternalServerError, res.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("bad data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", strings.NewReader(`{bad}`)).
			WithContext(context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials))

		res := httptest.NewRecorder()

		dataModelHandler := DataModelHandler{}
		handler := http.HandlerFunc(dataModelHandler.CreateTable)
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusBadRequest, res.Code)
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

	t.Run("nominal", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("tableID", tableID)

		ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

		body := `{"name": "name", "description": "description", "type": "Int", "nullable": true, "is_enum": true}`
		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com/{tableID}", strings.NewReader(body)).
			WithContext(ctx)

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateField", mock.Anything, organizationID, tableID, field).
			Return(fieldID, nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.CreateField)

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		expected := fmt.Sprintf(`{"id": "%s"}`, fieldID)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.JSONEq(t, expected, res.Body.String())
		mockUseCase.AssertExpectations(t)
	})

	t.Run("CreateField error", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("tableID", tableID)

		ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

		body := `{"name": "name", "description": "description", "type": "Int", "nullable": true, "is_enum": true}`
		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com/{tableID}", strings.NewReader(body)).
			WithContext(ctx)

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateField", mock.Anything, organizationID, tableID, field).
			Return("", assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.CreateField)

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusInternalServerError, res.Code)
		mockUseCase.AssertExpectations(t)
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

	t.Run("nominal", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com/{tableID}", strings.NewReader(body)).
			WithContext(ctx)

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateDataModelLink", mock.Anything, link).
			Return(nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.CreateLink)

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusNoContent, res.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("nominal", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com/{tableID}", strings.NewReader(body)).
			WithContext(ctx)

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("CreateDataModelLink", mock.Anything, link).
			Return(assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.CreateLink)

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusInternalServerError, res.Code)
		mockUseCase.AssertExpectations(t)
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

	t.Run("nominal", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("tableID", tableID)

		ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com/{tableID}", strings.NewReader(body)).
			WithContext(ctx)

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("UpdateDataModelTable", mock.Anything, tableID, description).
			Return(nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.UpdateDataModelTable)

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusNoContent, res.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("UpdateDataModelTable error", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("tableID", tableID)

		ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com/{tableID}", strings.NewReader(body)).
			WithContext(ctx)

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("UpdateDataModelTable", mock.Anything, tableID, description).
			Return(assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.UpdateDataModelTable)

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusInternalServerError, res.Code)
		mockUseCase.AssertExpectations(t)
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

	t.Run("nominal", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("fieldID", fieldID)

		ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com/{tableID}", strings.NewReader(body)).
			WithContext(ctx)

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("UpdateDataModelField", mock.Anything, fieldID, models.UpdateDataModelFieldInput{Description: &description, IsEnum: nil}).
			Return(nil)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.UpdateDataModelField)

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusNoContent, res.Code)
		mockUseCase.AssertExpectations(t)
	})

	t.Run("UpdateDataModelField error", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("fieldID", fieldID)

		ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com/{tableID}", strings.NewReader(body)).
			WithContext(ctx)

		mockUseCase := new(mocks.DataModelUseCase)
		mockUseCase.On("UpdateDataModelField", mock.Anything, fieldID, models.UpdateDataModelFieldInput{Description: &description, IsEnum: nil}).
			Return(assert.AnError)

		dataModelHandler := DataModelHandler{
			useCase: mockUseCase,
		}
		handler := http.HandlerFunc(dataModelHandler.UpdateDataModelField)

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusInternalServerError, res.Code)
		mockUseCase.AssertExpectations(t)
	})
}