package api

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/models"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid"
)

func PresentModel(w http.ResponseWriter, model any) {
	err := json.NewEncoder(w).Encode(model)
	if err != nil {
		panic(err)
	}

}

func PresentNothing(w http.ResponseWriter) {
	w.Header().Del("Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func validateUuid(uuidParam string) error {
	_, err := uuid.FromString(uuidParam)
	if err != nil {
		err = fmt.Errorf("'%s' is not a valid UUID: %w", uuidParam, models.BadParameterError)
	}
	return err
}

func requiredUuidUrlParam(r *http.Request, urlParamName string) (string, error) {
	uuidParam := chi.URLParam(r, urlParamName)
	if uuidParam == "" {
		return "", fmt.Errorf("Url Param '%s' is required: %w", urlParamName, models.BadParameterError)
	}

	if err := validateUuid(uuidParam); err != nil {
		return "", err
	}
	return uuidParam, nil
}
