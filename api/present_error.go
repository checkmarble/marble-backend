package api

import (
	. "marble/marble-backend/models"
	"net/http"
)

func presentError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*BadParameterError); ok {
		http.Error(w, e.Error(), http.StatusBadRequest)
	} else if e, ok := err.(*UnAuthorizedError); ok {
		http.Error(w, e.Error(), http.StatusUnauthorized)
	} else if e, ok := err.(*NotFoundError); ok {
		http.Error(w, e.Error(), http.StatusNotFound)
	} else {
		panic(err)
	}
	return true
}
