package payload_parser

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/tidwall/gjson"

	"github.com/checkmarble/marble-backend/models"
)

type fieldParser map[models.DataType]func(result gjson.Result) (any, error)

type Parser struct {
	parsers    fieldParser
	allowPatch bool
}

var (
	errIsInvalidJSON      = fmt.Errorf("json is invalid")
	errIsNotNullable      = fmt.Errorf("is not nullable")
	errIsInvalidTimestamp = fmt.Errorf("is not a valid timestamp")
	errIsInvalidInteger   = fmt.Errorf("is not a valid integer")
	errIsInvalidFloat     = fmt.Errorf("is not a valid float")
	errIsInvalidBoolean   = fmt.Errorf("is not a valid boolean")
	errIsInvalidString    = fmt.Errorf("is not a valid string")
	errIsInvalidDataType  = fmt.Errorf("invalid type used in parser")
)

func addError(errorsContainer models.IngestionValidationErrors, objectId string, name string, err error) {
	if _, ok := errorsContainer[objectId]; !ok {
		errorsContainer[objectId] = make(map[string]string)
	}
	errorsContainer[objectId][name] = err.Error()
}

func (p *Parser) ParsePayload(table models.Table, json []byte) (models.ClientObject, error) {
	if !gjson.ValidBytes(json) {
		return models.ClientObject{}, errors.Join(models.BadParameterError, errIsInvalidJSON)
	}

	allErrors := make(models.IngestionValidationErrors)
	out := make(map[string]any)
	result := gjson.ParseBytes(json)
	missingFields := make([]models.MissingField, 0, len(table.Fields))

	// different treatment for object_id, because its value should not be an empty string and is required to construct the validation errors below
	objectIdRes := result.Get("object_id")
	objectId := objectIdRes.String()
	if !objectIdRes.Exists() || objectIdRes.Type == gjson.Null || objectIdRes.String() == "" {
		addError(allErrors, objectId, "object_id", errIsNotNullable)
	}

	for name, field := range table.Fields {
		value := result.Get(name)
		if !value.Exists() {
			// specific case for updated_at which is always required, because it is necessary for proper ingestion at the repository level
			if p.allowPatch && name != "updated_at" {
				missingFields = append(missingFields, models.MissingField{
					Field:          field,
					ErrorIfMissing: errIsNotNullable.Error(),
				})
			} else if !field.Nullable {
				addError(allErrors, objectId, name, errIsNotNullable)
			}
			continue
		}

		if value.Type == gjson.Null {
			if !field.Nullable {
				addError(allErrors, objectId, name, errIsNotNullable)
			}
			out[name] = nil
			continue
		}

		parseField, ok := p.parsers[field.DataType]
		if !ok {
			return models.ClientObject{}, fmt.Errorf("%w: %s",
				errIsInvalidDataType, field.DataType.String())
		}
		if val, err := parseField(value); err != nil {
			addError(allErrors, objectId, name, err)
		} else {
			out[name] = val
		}
	}
	if len(allErrors) > 0 {
		return models.ClientObject{}, allErrors
	}
	return models.ClientObject{
		TableName:             table.Name,
		Data:                  out,
		MissingFieldsToLookup: missingFields,
	}, nil
}

type parserOpts struct {
	allowPatch bool
}

type ParserOpt func(*parserOpts)

func WithAllowPatch() ParserOpt {
	return func(o *parserOpts) {
		o.allowPatch = true
	}
}

func NewParser(opts ...ParserOpt) *Parser {
	parsers := fieldParser{
		models.Timestamp: func(result gjson.Result) (any, error) {
			if t, err := time.Parse(time.RFC3339, result.String()); err == nil {
				return t.UTC(), nil
			}
			if t, err := time.Parse("2006-01-02 15:04:05.9", result.String()); err == nil {
				return t.UTC(), nil
			}
			return nil, fmt.Errorf("%w: expected format \"YYYY-MM-DD hh:mm:ss[+optional decimals]\" or \"YYYY-MM-DDThh:mm:ss[+optional decimals]Z\", got \"%s\"", errIsInvalidTimestamp, result.String())
		},
		models.Int: func(result gjson.Result) (any, error) {
			i, err := strconv.ParseInt(result.Raw, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: expected an integer, got %s", errIsInvalidInteger, result.Raw)
			}
			return i, nil
		},
		models.Float: func(result gjson.Result) (any, error) {
			f, err := strconv.ParseFloat(result.Raw, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: expected a float, got %s", errIsInvalidFloat, result.Raw)
			}
			return f, nil
		},
		models.String: func(result gjson.Result) (any, error) {
			if result.Type != gjson.String {
				return nil, errIsInvalidString
			}
			return result.String(), nil
		},
		models.Bool: func(result gjson.Result) (any, error) {
			if !result.IsBool() {
				return nil, fmt.Errorf("%w: expected a boolean, got %s", errIsInvalidBoolean, result.Raw)
			}
			return result.Bool(), nil
		},
	}

	options := parserOpts{}
	for _, opt := range opts {
		opt(&options)
	}

	return &Parser{
		parsers:    parsers,
		allowPatch: options.allowPatch,
	}
}
