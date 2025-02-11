package payload_parser

import (
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

func addError(errorsContainer models.IngestionValidationErrorsMultiple, objectId string, name string, err error) {
	if _, ok := errorsContainer[objectId]; !ok {
		errorsContainer[objectId] = make(map[string]string)
	}
	errorsContainer[objectId][name] = err.Error()
}

func (p *Parser) ParsePayload(table models.Table, json []byte) (models.ClientObject, models.IngestionValidationErrorsMultiple, error) {
	if !gjson.ValidBytes(json) {
		return models.ClientObject{}, nil, errIsInvalidJSON
	}

	allErrors := make(models.IngestionValidationErrorsMultiple)
	out := make(map[string]any)
	result := gjson.ParseBytes(json)
	missingFields := make([]models.MissingField, 0, len(table.Fields))

	objectId := ""
	objectIdRes := result.Get("object_id")
	if !objectIdRes.Exists() || objectIdRes.Type == gjson.Null || objectIdRes.String() == "" {
		objectId = ""
		addError(allErrors, objectId, "object_id", errIsNotNullable)
	}
	objectId = objectIdRes.String()

	for name, field := range table.Fields {
		value := result.Get(name)
		if !value.Exists() {
			if p.allowPatch {
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
			return models.ClientObject{}, nil, fmt.Errorf("%w: %s",
				errIsInvalidDataType, field.DataType.String())
		}
		if val, err := parseField(value); err != nil {
			addError(allErrors, objectId, name, err)
		} else {
			out[name] = val
		}
	}
	if len(allErrors) > 0 {
		return models.ClientObject{}, allErrors, nil
	}
	return models.ClientObject{
		TableName:             table.Name,
		Data:                  out,
		MissingFieldsToLookup: missingFields,
	}, nil, nil
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
