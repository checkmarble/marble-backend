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
	parsers fieldParser
}

var (
	errIsInvalidJSON      = fmt.Errorf("json is invalid")
	errIsNotNullable      = fmt.Errorf("is not nullable")
	errIsInvalidTimestamp = fmt.Errorf("is not a valid timestamp")
	errIsInvalidInteger   = fmt.Errorf("is not a valid integer")
	errIsInvalidFloat     = fmt.Errorf("is not a valid float")
	errIsInvalidBoolean   = fmt.Errorf("is not a valid boolean")
	errIsInvalidString    = fmt.Errorf("is not a valid string")
	errIsInvalidDataType  = fmt.Errorf("invalid type")
)

func (p *Parser) ParsePayload(table models.Table, json []byte) (models.ClientObject, map[string]string, error) {
	if !gjson.ValidBytes(json) {
		return models.ClientObject{}, nil, errIsInvalidJSON
	}

	errors := make(map[string]string)
	out := make(map[string]any)
	result := gjson.ParseBytes(json)

	// Check fields that are always mandatory, regardless of the table definition
	for _, name := range []string{"object_id", "updated_at"} {
		value := result.Get(name)
		if !value.Exists() || value.Type == gjson.Null {
			errors[name] = errIsNotNullable.Error()
		}
		if name == "object_id" && result.String() == "" {
			errors[name] = errIsNotNullable.Error()
		}
	}

	for name, field := range table.Fields {
		value := result.Get(name)
		if !value.Exists() || value.Type == gjson.Null {
			if !field.Nullable {
				errors[name] = errIsNotNullable.Error()
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
			errors[name] = err.Error()
		} else {
			out[name] = val
		}
	}
	if len(errors) > 0 {
		return models.ClientObject{}, errors, nil
	}
	return models.ClientObject{
		TableName: table.Name,
		Data:      out,
	}, nil, nil
}

func NewParser() *Parser {
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

	return &Parser{
		parsers: parsers,
	}
}
