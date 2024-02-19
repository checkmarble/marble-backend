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

func (p *Parser) ParsePayload(table models.Table, json []byte) (models.ClientObject, map[models.FieldName]string, error) {
	if !gjson.ValidBytes(json) {
		return models.ClientObject{}, nil, errIsInvalidJSON
	}

	errors := make(map[models.FieldName]string)
	out := make(map[string]any)
	result := gjson.ParseBytes(json)

	// Check fields that are always mandatory, regardless of the table definition
	for _, name := range []string{"object_id", "updated_at"} {
		value := result.Get(name)
		if !value.Exists() || value.Type == gjson.Null {
			errors[models.FieldName(name)] = errIsNotNullable.Error()
		}
		if name == "object_id" && result.String() == "" {
			errors[models.FieldName(name)] = errIsNotNullable.Error()
		}
	}

	for name, field := range table.Fields {
		value := result.Get(string(name))
		if !value.Exists() || value.Type == gjson.Null {
			if !field.Nullable {
				errors[name] = errIsNotNullable.Error()
			}
			out[string(name)] = nil
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
			out[string(name)] = val
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
	validators := fieldParser{
		models.Timestamp: func(result gjson.Result) (any, error) {
			t, err1 := time.Parse(time.RFC3339, result.String())
			if err1 == nil {
				return t.UTC(), nil
			}
			t, err2 := time.Parse("2006-01-02 15:04:05.9", result.String())
			if err2 != nil {
				return nil, fmt.Errorf("%w: expected format \"YYYY-MM-DD hh:mm:ss[+optional decimals]\" or \"YYYY-MM-DDThh:mm:ss[+optional decimals]Z\", got \"%s\"", errIsInvalidTimestamp, result.String())
			}
			return t.UTC(), nil
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
		parsers: validators,
	}
}
