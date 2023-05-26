package api

import (
	"errors"
	"fmt"

	. "marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
)

func presentError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, BadParameterError) {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else if errors.Is(err, UnAuthorizedError) {
		http.Error(w, err.Error(), http.StatusUnauthorized)
	} else if errors.Is(err, ForbiddenError) {
		http.Error(w, err.Error(), http.StatusForbidden)
	} else if errors.Is(err, NotFoundError) {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		utils.LogRequestError(r, fmt.Sprintf("Unexpected Error: %s", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return true
}
