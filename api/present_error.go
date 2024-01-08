package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

func presentError(w http.ResponseWriter, r *http.Request, err error, c *gin.Context) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, models.BadParameterError) {
		utils.LogRequestInfo(r, fmt.Sprintf("BadParameterError: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)

	} else if errors.Is(err, models.UnAuthorizedError) {
		utils.LogRequestInfo(r, fmt.Sprintf("UnAuthorizedError: %v", err))
		http.Error(w, err.Error(), http.StatusUnauthorized)

	} else if errors.Is(err, models.ForbiddenError) {
		utils.LogRequestInfo(r, fmt.Sprintf("ForbiddenError: %v", err))
		http.Error(w, err.Error(), http.StatusForbidden)

	} else if errors.Is(err, models.NotFoundError) {
		utils.LogRequestInfo(r, fmt.Sprintf("NotFoundError: %v", err))
		http.Error(w, err.Error(), http.StatusNotFound)

	} else if errors.Is(err, models.DuplicateValueError) {
		utils.LogRequestInfo(r, fmt.Sprintf("DuplicateValueError: %v", err))
		http.Error(w, err.Error(), http.StatusConflict)

	} else {
		utils.LogRequestError(r, fmt.Sprintf("Unexpected Error: %+v", err))
		if hub := sentrygin.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CaptureException(err)
		}
		http.Error(w, "", http.StatusInternalServerError)
	}
	return true
}
