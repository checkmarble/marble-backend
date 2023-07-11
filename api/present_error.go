package api

import (
	"errors"

	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
)

func presentError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, models.BadParameterError) {
		utils.LogRequestError(r, "BadParameterError", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)

	} else if errors.Is(err, models.UnAuthorizedError) {
		utils.LogRequestError(r, "UnAuthorizedError", "error", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)

	} else if errors.Is(err, models.ForbiddenError) {
		utils.LogRequestError(r, "ForbiddenError", "error", err)
		http.Error(w, err.Error(), http.StatusForbidden)

	} else if errors.Is(err, models.NotFoundError) {
		utils.LogRequestError(r, "NotFoundError", "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)

	} else {
		utils.LogRequestError(r, "Unexpected Error", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return true
}
