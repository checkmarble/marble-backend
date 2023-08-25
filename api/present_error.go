package api

import (
	"errors"
	"fmt"

	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
)

func presentError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, models.BadParameterError) {
		utils.LogRequestError(r, fmt.Sprintf("BadParameterError: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)

	} else if errors.Is(err, models.UnAuthorizedError) {
		utils.LogRequestError(r, fmt.Sprintf("UnAuthorizedError: %v", err))
		http.Error(w, err.Error(), http.StatusUnauthorized)

	} else if errors.Is(err, models.ForbiddenError) {
		utils.LogRequestError(r, fmt.Sprintf("ForbiddenError: %v", err))
		http.Error(w, err.Error(), http.StatusForbidden)

	} else if errors.Is(err, models.NotFoundError) {
		utils.LogRequestError(r, fmt.Sprintf("NotFoundError: %v", err))
		http.Error(w, err.Error(), http.StatusNotFound)

	} else if errors.Is(err, models.DuplicateValueError) {
		utils.LogRequestError(r, fmt.Sprintf("DuplicateValueError: %v", err))
		http.Error(w, err.Error(), http.StatusConflict)

	} else {
		utils.LogRequestError(r, fmt.Sprintf("Unexpected Error: %+v", err))
		http.Error(w, "", http.StatusInternalServerError)
	}
	return true
}
