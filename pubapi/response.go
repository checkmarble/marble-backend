package pubapi

import (
	"io"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"

	ut "github.com/go-playground/universal-translator"
)

var validationTranslator ut.Translator

type baseResponse[T any] struct {
	Data  *T               `json:"data,omitempty"`
	Links map[string][]any `json:"links,omitempty"`
}

type baseErrorResponse struct {
	Error ErrorResponse `json:"error"`
}

type ErrorResponse struct {
	err error `json:"-"`

	Code    string   `json:"code,omitempty"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

func NewResponse[T any](data T) baseResponse[T] {
	return baseResponse[T]{
		Data: &data,
	}
}

func (resp baseResponse[T]) WithLink(kind string, value any) baseResponse[T] {
	if resp.Links == nil {
		resp.Links = make(map[string][]any)
	}
	if _, ok := resp.Links[kind]; !ok {
		resp.Links[kind] = make([]any, 0)
	}

	resp.Links[kind] = append(resp.Links[kind], value)

	return resp
}

func NewErrorResponse() baseErrorResponse {
	return baseErrorResponse{
		Error: ErrorResponse{
			Message: ErrInternalServerError.Error(),
		},
	}
}

func (resp baseErrorResponse) WithErrorCode(code string) baseErrorResponse {
	resp.Error.Code = code

	return resp
}

func (resp baseErrorResponse) WithError(err error) baseErrorResponse {
	switch err := err.(type) { // nolint:errorlint
	case validator.ValidationErrors:
		resp.Error.err = ErrInvalidPayload
		resp.Error.Message = ErrInvalidPayload.Error()

		resp.Error.Details = pure_utils.Map(err, func(verr validator.FieldError) string {
			return verr.Translate(validationTranslator)
		})

	default:
		switch {
		case errors.Is(err, io.EOF):
			resp.Error.err = ErrMissingPayload
			resp.Error.Message = ErrMissingPayload.Error()

		case
			errors.Is(err, models.NotFoundError),
			errors.Is(err, pgx.ErrNoRows):

			resp.Error.err = err
			resp.Error.Message = "requested resource was not found"

		case
			errors.Is(err, ErrFeatureDisabled),
			errors.Is(err, ErrNotConfigured):

			resp.Error.err = err
			resp.Error.Message = "feature not available"

		case
			errors.Is(err, ErrInvalidId),
			errors.Is(err, ErrInvalidPayload),
			errors.Is(err, ErrMissingPayload):

			resp.Error.err = err
			resp.Error.Message = "invalid parameters or payload"

		case
			errors.Is(err, models.UnprocessableEntityError),
			errors.Is(err, ErrUnprocessableEntity):

			resp.Error.status = http.StatusUnprocessableEntity
			resp.Error.Code = ErrUnprocessableEntity.Error()

		default:
			resp.Error.err = err
			resp.Error.Message = ErrInternalServerError.Error()
		}
	}

	if details := errors.GetAllDetails(err); len(details) > 0 {
		resp.Error.Details = append(resp.Error.Details, details...)
	}

	return resp
}

func (resp baseErrorResponse) WithErrorMessage(message string) baseErrorResponse {
	resp.Error.Message = message

	return resp
}

func (resp baseErrorResponse) WithErrorDetails(details ...string) baseErrorResponse {
	resp.Error.Details = details

	return resp
}

func (resp baseResponse[T]) Serve(c *gin.Context, statuses ...int) {
	status := http.StatusOK
	if len(statuses) > 0 {
		status = statuses[0]
	}

	c.JSON(status, resp)
}

func (resp baseErrorResponse) Serve(c *gin.Context, statuses ...int) {
	status := http.StatusInternalServerError

	switch {
	case len(statuses) > 0:
		status = statuses[0]

	case resp.Error.err != nil:
		switch {
		case
			errors.Is(resp.Error.err, ErrInvalidId),
			errors.Is(resp.Error.err, ErrMissingPayload),
			errors.Is(resp.Error.err, ErrInvalidPayload),
			errors.Is(resp.Error.err, models.BadParameterError):
			status = http.StatusBadRequest

		case
			errors.Is(resp.Error.err, models.UnAuthorizedError),
			errors.Is(resp.Error.err, models.ForbiddenError):
			status = http.StatusUnauthorized

		case
			errors.Is(resp.Error.err, models.NotFoundError),
			errors.Is(resp.Error.err, pgx.ErrNoRows):
			status = http.StatusNotFound

		case errors.Is(resp.Error.err, models.ConflictError):
			status = http.StatusConflict

		case errors.Is(resp.Error.err, ErrFeatureDisabled):
			status = http.StatusPaymentRequired

		case errors.Is(resp.Error.err, ErrNotConfigured):
			status = http.StatusNotImplemented
		}
	}

	c.JSON(status, resp)
}
