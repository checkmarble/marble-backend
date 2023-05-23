package api

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/exp/slog"

	. "marble/marble-backend/models"
	"net/http"
)

func presentError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, err error) bool {
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
		logger.ErrorCtx(ctx, fmt.Sprintf("Unexpected Error: %s", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return true
}
