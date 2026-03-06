package types

import (
	"errors"

	gdto "github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"

	"github.com/gin-gonic/gin"
)

func PresentMultipleObjectsValidationError(c *gin.Context, err error) bool {
	var validationError models.IngestionValidationErrors

	if errors.As(err, &validationError) {
		NewErrorResponse().
			WithError(err).
			WithErrorCode(string(gdto.SchemaMismatchError)).
			WithErrorMessage("one or more provided trigger objects are invalid").
			WithErrorDetails(validationError).
			Serve(c)

		return true
	}

	return false
}

func PresentSingleObjectValidationError(c *gin.Context, err error) bool {
	var validationError models.IngestionValidationErrors

	if errors.As(err, &validationError) {
		_, errs := validationError.GetSomeItem()

		NewErrorResponse().
			WithError(errs).
			WithErrorCode(string(gdto.SchemaMismatchError)).
			WithErrorMessage("the provided trigger object is invalid").
			WithErrorDetails(errs).
			Serve(c)

		return true
	}

	return false
}
