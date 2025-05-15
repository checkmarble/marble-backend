package pubapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
)

// var validationTranslator ut.Translator

type baseResponse[T any] struct {
	Data       *T                  `json:"data,omitempty"`
	Links      map[string][]any    `json:"-"`
	Pagination *paginationResponse `json:"pagination,omitempty"`
}

type paginationResponse struct {
	HasMore    bool   `json:"has_more"`
	NextPageId string `json:"next_page_id,omitempty"`
}

type baseErrorResponse struct {
	Error ErrorResponse `json:"error"`
}

type ErrorResponse struct {
	err    error `json:"-"`
	status int   `json:"-"`

	Code     string          `json:"code"`
	Messages []string        `json:"messages,omitempty"`
	Detail   json.RawMessage `json:"detail,omitempty"`
}

func NewResponse[T any](data T) baseResponse[T] {
	return baseResponse[T]{
		Data: &data,
	}
}

func (resp baseResponse[T]) WithPagination(hasMore bool, nextPageId string) baseResponse[T] {
	if hasMore {
		resp.Pagination = &paginationResponse{
			HasMore:    hasMore,
			NextPageId: nextPageId,
		}
	}

	return resp
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
			Code: ErrInternalServerError.Error(),
		},
	}
}

func (resp baseErrorResponse) WithErrorCode(code string) baseErrorResponse {
	resp.Error.Code = code

	return resp
}

func (resp baseErrorResponse) WithError(err error) baseErrorResponse {
	resp.Error.err = err
	resp.Error.status = http.StatusInternalServerError

	switch err := err.(type) { // nolint:errorlint
	case validator.ValidationErrors:
		resp.Error.status = http.StatusBadRequest
		resp.Error.Code = ErrInvalidPayload.Error()
		resp.Error.Messages = pure_utils.Map(err, func(verr validator.FieldError) string {
			return AdaptFieldValidationError(verr)
		})

	// Special case when the validation error comes from the JSON itself not unmarshaling.
	// Turns these:
	//   - json: cannot unmarshal string into Go struct field .age of type int
	// into:
	//   - field `age` expected a int, got string
	case *json.UnmarshalTypeError:
		resp.Error.status = http.StatusBadRequest
		resp.Error.Code = ErrInvalidPayload.Error()

		msg := fmt.Sprintf("expected type %s field, got type `%s`", err.Type.String(), err.Value)
		if err.Field != "" {
			msg = fmt.Sprintf("field `%s` expected type %s, got type %s", err.Field, err.Type.String(), err.Value)
		}

		resp.Error.Messages = []string{msg}

	default:
		switch {
		case errors.Is(err, models.ForbiddenError):
			resp.Error.status = http.StatusForbidden
			resp.Error.Code = ErrForbidden.Error()

		case
			errors.Is(err, models.NotFoundError),
			errors.Is(err, pgx.ErrNoRows):

			resp.Error.status = http.StatusNotFound
			resp.Error.Code = ErrNotFound.Error()

		case errors.Is(err, models.ConflictError):
			resp.Error.status = http.StatusConflict
			resp.Error.Code = ErrConflict.Error()

		case errors.Is(err, ErrFeatureDisabled):
			resp.Error.status = http.StatusPaymentRequired
			resp.Error.Code = ErrFeatureDisabled.Error()

		case errors.Is(err, ErrNotConfigured):
			resp.Error.status = http.StatusNotImplemented
			resp.Error.Code = ErrNotConfigured.Error()

		case
			errors.Is(err, io.EOF),
			errors.Is(err, ErrInvalidPayload),
			errors.Is(err, models.BadParameterError):

			resp.Error.status = http.StatusBadRequest
			resp.Error.Code = ErrInvalidPayload.Error()

		case
			errors.Is(err, models.UnprocessableEntityError),
			errors.Is(err, ErrUnprocessableEntity):

			resp.Error.status = http.StatusUnprocessableEntity
			resp.Error.Code = ErrUnprocessableEntity.Error()

		default:
			resp.Error.err = err
		}
	}

	if details := errors.GetAllDetails(err); len(details) > 0 {
		resp.Error.Messages = append(resp.Error.Messages, details...)
	}

	return resp
}

func (resp baseErrorResponse) WithErrorMessage(message string) baseErrorResponse {
	resp.Error.Code = message

	return resp
}

func (resp baseErrorResponse) WithErrorDetails(details ...string) baseErrorResponse {
	resp.Error.Messages = details

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
	c.JSON(resp.Error.status, resp)
}
