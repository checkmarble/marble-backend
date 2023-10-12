package api

import (
	"net/http"

	"github.com/cockroachdb/errors"
)

func HandleCrash(w http.ResponseWriter, r *http.Request) {
	err := errors.New("Voluntary crash for test endpoint")
	if presentError(w, r, err) {
		return
	}
}
