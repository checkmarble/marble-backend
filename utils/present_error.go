package utils

import (
	"errors"
	"fmt"

	. "marble/marble-backend/models"
	"net/http"
)

func PresentError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, BadParameterError) {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else if errors.Is(err, UnAuthorizedError) {
		LogRequestError(r, fmt.Sprintf("UnAuthorizedError: %v", err))
		http.Error(w, err.Error(), http.StatusUnauthorized)
	} else if errors.Is(err, ForbiddenError) {
		LogRequestError(r, fmt.Sprintf("ForbiddenError: %v", err))
		http.Error(w, err.Error(), http.StatusForbidden)
	} else if errors.Is(err, NotFoundError) {
		LogRequestError(r, fmt.Sprintf("NotFoundError: %v", err))
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		LogRequestError(r, fmt.Sprintf("Unexpected Error: %v", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return true
}
