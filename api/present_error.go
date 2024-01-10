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

func presentError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, models.BadParameterError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("BadParameterError: %v", err))
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)

	} else if errors.Is(err, models.UnAuthorizedError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("UnAuthorizedError: %v", err))
		http.Error(c.Writer, err.Error(), http.StatusUnauthorized)

	} else if errors.Is(err, models.ForbiddenError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("ForbiddenError: %v", err))
		http.Error(c.Writer, err.Error(), http.StatusForbidden)

	} else if errors.Is(err, models.NotFoundError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("NotFoundError: %v", err))
		http.Error(c.Writer, err.Error(), http.StatusNotFound)

	} else if errors.Is(err, models.DuplicateValueError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("DuplicateValueError: %v", err))
		http.Error(c.Writer, err.Error(), http.StatusConflict)

	} else {
		utils.LogRequestError(c.Request, fmt.Sprintf("Unexpected Error: %+v", err))
		if hub := sentrygin.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CaptureException(err)
		}
		http.Error(c.Writer, "", http.StatusInternalServerError)
	}
	return true
}
