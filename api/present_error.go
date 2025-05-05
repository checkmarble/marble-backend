package api

import (
	"context"
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

const timeoutMst = "Sorry, the API timed out. Please try again later."

func presentError(ctx context.Context, c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	errorResponse := dto.APIErrorResponse{
		Message: err.Error(),
	}
	logger := utils.LoggerFromContext(ctx)

	switch {
	case errors.Is(err, models.BadParameterError):
		logger.InfoContext(ctx, fmt.Sprintf("BadParameterError: %v", err))
		c.JSON(http.StatusBadRequest, errorResponse)
	case errors.Is(err, models.UnprocessableEntityError):
		logger.InfoContext(ctx, fmt.Sprintf("UnprocessableEntityError: %v", err))
		c.JSON(http.StatusUnprocessableEntity, errorResponse)
	case errors.Is(err, models.UnAuthorizedError):
		logger.InfoContext(ctx, fmt.Sprintf("UnAuthorizedError: %v", err))
		c.JSON(http.StatusUnauthorized, errorResponse)
	case errors.Is(err, models.ForbiddenError):
		logger.InfoContext(ctx, fmt.Sprintf("ForbiddenError: %v", err))
		c.JSON(http.StatusForbidden, errorResponse)
	case errors.Is(err, models.NotFoundError):
		logger.InfoContext(ctx, fmt.Sprintf("NotFoundError: %v", err))
		c.JSON(http.StatusNotFound, errorResponse)
	case errors.Is(err, models.ConflictError):
		logger.InfoContext(ctx, fmt.Sprintf("ConflictError: %v", err))
		c.JSON(http.StatusConflict, errorResponse)
	case errors.Is(err, models.MissingRequirementError{}):
		var req models.MissingRequirementError

		if errors.As(err, &req) {
			logger.InfoContext(ctx, fmt.Sprintf("MissingRequirementError: %v", err))

			errorResponse = dto.APIErrorResponse{
				ErrorCode: dto.MissingRequirement,
				Message:   "A required configuration was missing or invalid",
				Details: dto.RequirementErrorDto{
					Requirement: string(req.Requirement),
					Reason:      string(req.Reason),
					Error:       req.Err.Error(),
				},
			}

			c.JSON(http.StatusNotImplemented, errorResponse)
		}

	case errors.Is(err, context.DeadlineExceeded):
		logger.WarnContext(ctx, fmt.Sprintf("Deadline exceeded: %v", err))
		c.JSON(http.StatusRequestTimeout, dto.APIErrorResponse{Message: timeoutMst})
	case errors.Is(err, context.Canceled):
		logger.WarnContext(ctx, fmt.Sprintf("Context canceled: %v", err))
		c.JSON(http.StatusRequestTimeout, dto.APIErrorResponse{Message: timeoutMst})

	default:
		logger.ErrorContext(ctx, fmt.Sprintf("Unexpected Error: %+v", err))
		if hub := sentrygin.GetHubFromContext(c); hub != nil {
			utils.CaptureSentryException(ctx, hub, err)
		} else {
			sentry.CaptureException(err)
		}
		c.JSON(http.StatusInternalServerError, dto.APIErrorResponse{
			Message: "An unexpected error occurred. Please try again later, or contact support if the problem persists.",
		})
	}
	return true
}
