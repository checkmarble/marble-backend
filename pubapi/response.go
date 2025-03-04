package pubapi

import (
	"errors"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	entrans "github.com/go-playground/validator/v10/translations/en"
)

var validationTranslator ut.Translator

func init() {
	if validator, ok := binding.Validator.Engine().(*validator.Validate); ok {
		en := en.New()
		uni := ut.New(en, en)
		validationTranslator, _ = uni.GetTranslator("en")
		_ = entrans.RegisterDefaultTranslations(validator, validationTranslator)
	}
}

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
		resp.Error.err = err
		resp.Error.Message = err.Error()
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
			errors.Is(resp.Error.err, ErrInvalidPayload):
			status = http.StatusBadRequest

		case errors.Is(resp.Error.err, models.NotFoundError):
			status = http.StatusNotFound

		case errors.Is(resp.Error.err, ErrFeatureDisabled):
			status = http.StatusPaymentRequired

		case errors.Is(resp.Error.err, ErrNotConfigured):
			status = http.StatusNotImplemented
		}
	}

	c.JSON(status, resp)
}
