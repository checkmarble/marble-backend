package pubapi

// Authored by Antoine Popineau
// MIT license

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// AdaptFieldValidationError maps generic validation error to human-readable error
// messages, to be returned in the response.
func AdaptFieldValidationError(fe validator.FieldError) string {
	inner := func(fe validator.FieldError) string {
		switch fe.ActualTag() {
		case "required":
			return "is required"
		case "oneof":
			opts := strings.Split(fe.Param(), " ")

			return fmt.Sprintf("must be one of %s", strings.Join(opts, ", "))
		case "gt":
			if fe.Param() == "0" {
				return "must not be empty"
			}

			if reflect.TypeOf(fe.Value()).Kind() == reflect.Slice {
				return fmt.Sprintf("must have more than %s items", fe.Param())
			}
			if _, ok := fe.Value().(int); ok {
				return fmt.Sprintf("must be greater than %s", fe.Param())
			}
			return fmt.Sprintf("must have more than %s character", fe.Param())
		case "gtcsfield":
			return fmt.Sprintf("must be greater than the value of `%s`", fe.Param())
		case "ltcsfield":
			return fmt.Sprintf("must be less than the value of `%s`", fe.Param())
		case "lt":
			if reflect.TypeOf(fe.Value()).Kind() == reflect.Slice {
				return fmt.Sprintf("must have less than %s items", fe.Param())
			}
			if _, ok := fe.Value().(int); ok {
				return fmt.Sprintf("must be less than %s", fe.Param())
			}
			return fmt.Sprintf("must have less than %s character", fe.Param())
		case "lte":
			if _, ok := fe.Value().([]any); ok {
				return fmt.Sprintf("must have at most %s items", fe.Param())
			}
			if _, ok := fe.Value().(int); ok {
				return fmt.Sprintf("must be at most %s", fe.Param())
			}
			return fmt.Sprintf("must have at most %s character", fe.Param())
		case "len":
			return fmt.Sprintf("must be of length %s", fe.Param())
		case "boolean":
			return "should be 'true' or 'false'"
		case "datetime":
			return fmt.Sprintf("should be in the format '%s'", time.RFC3339)
		case "required_with_all", "required_if":
			return "is required when using the other parameters in your query"
		case "required_with":
			return fmt.Sprintf("is required when used with %s", fe.Param())
		case "required_without_all":
			return fmt.Sprintf("should be provided if none of %s is provided",
				strings.ReplaceAll(fe.Param(), " ", ", "))
		case "excluded_with":
			return fmt.Sprintf("cannot be provided if one of %s is present",
				strings.ReplaceAll(fe.Param(), " ", ", "))
		case "excluded_unless":
			return fmt.Sprintf("cannot be provided unless %s", fe.Param())
		case "uuid":
			return "should be a UUID"
		}

		return "is invalid"
	}

	return fmt.Sprintf("field `%s` %s", fe.Field(), inner(fe))
}
