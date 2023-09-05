package api

import (
	"encoding/json"
	"fmt"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func PresentModel(w http.ResponseWriter, model any) {
	err := json.NewEncoder(w).Encode(model)
	if err != nil {
		panic(err)
	}
}

func PresentModelWithName(w http.ResponseWriter, keyName string, models any) {
	PresentModel(w, map[string]any{
		keyName: models,
	})
}

func PresentModelWithNameStatusCode(w http.ResponseWriter, keyName string, models any, statusCode int) {
	w.WriteHeader(statusCode)
	PresentModel(w, map[string]any{
		keyName: models,
	})
}

func PresentNothing(w http.ResponseWriter) {
	PresentNothingStatusCode(w, http.StatusNoContent)
}

func PresentNothingStatusCode(w http.ResponseWriter, statusCode int) {
	w.Header().Del("Content-Type")
	w.WriteHeader(statusCode)
}

func requiredUuidUrlParam(r *http.Request, urlParamName string) (string, error) {
	uuidParam := chi.URLParam(r, urlParamName)
	if uuidParam == "" {
		return "", fmt.Errorf("search param '%s' is required: %w", urlParamName, models.BadParameterError)
	}

	if err := utils.ValidateUuid(uuidParam); err != nil {
		return "", fmt.Errorf("search param '%s' is not a valid uuid: %w", urlParamName, models.BadParameterError)
	}
	return uuidParam, nil
}
