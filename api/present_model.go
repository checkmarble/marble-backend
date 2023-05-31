package api

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
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

func PresentNothing(w http.ResponseWriter) {
	w.Header().Del("Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func requiredUuidUrlParam(r *http.Request, urlParamName string) (string, error) {
	uuidParam := chi.URLParam(r, urlParamName)
	if uuidParam == "" {
		return "", fmt.Errorf("Url Param '%s' is required: %w", urlParamName, models.BadParameterError)
	}

	if err := utils.ValidateUuid(uuidParam); err != nil {
		return "", err
	}
	return uuidParam, nil
}
