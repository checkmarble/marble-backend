package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
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
	errString := err.Error()
	errorResponse := dto.APIErrorResponse{
		Message: errString,
	}

	if errors.Is(err, models.BadParameterError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("BadParameterError: %v", err))
		c.JSON(http.StatusBadRequest, errorResponse)

	} else if errors.Is(err, models.UnAuthorizedError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("UnAuthorizedError: %v", err))
		c.JSON(http.StatusUnauthorized, errorResponse)

	} else if errors.Is(err, models.ForbiddenError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("ForbiddenError: %v", err))
		c.JSON(http.StatusForbidden, errorResponse)

	} else if errors.Is(err, models.NotFoundError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("NotFoundError: %v", err))
		c.JSON(http.StatusNotFound, errorResponse)

	} else if errors.Is(err, models.ConflictError) {
		utils.LogRequestInfo(c.Request, fmt.Sprintf("ConflictError: %v", err))
		c.JSON(http.StatusConflict, errorResponse)

	} else {
		utils.LogRequestError(c.Request, fmt.Sprintf("Unexpected Error: %+v", err))
		if hub := sentrygin.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CaptureException(err)
		}
		c.JSON(http.StatusInternalServerError, dto.APIErrorResponse{
			Message: "An unexpected error occurred. Please try again later, or contact support if the problem persists.",
		})
	}
	return true
}
