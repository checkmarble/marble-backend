package payload_parser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/twpayne/go-geos"
)

type fieldParser map[models.DataType]func(result gjson.Result) (any, error)

type Parser struct {
	parsers               fieldParser
	allowPatch            bool
	disallowUnknownFields bool
	columnEscape          bool
	enricher              PayloadEnrichementUsecase
}

var (
	errIsInvalidJSON      = fmt.Errorf("json is invalid")
	errIsNotNullable      = fmt.Errorf("is not nullable")
	errIsInvalidTimestamp = fmt.Errorf("is not a valid timestamp")
	errIsInvalidInteger   = fmt.Errorf("is not a valid integer")
	errIsInvalidFloat     = fmt.Errorf("is not a valid float")
	errIsInvalidBoolean   = fmt.Errorf("is not a valid boolean")
	errIsInvalidString    = fmt.Errorf("is not a valid string")
	errIsInvalidIpAddress = fmt.Errorf("is not a valid IP address")
	errIsInvalidCoords    = fmt.Errorf("are not valid coordinates (lat,lng)")
	errIsInvalidDataType  = fmt.Errorf("invalid type used in parser")
	errUnknownField       = errors.New("field does not exist in data model")
)

func addError(errorsContainer models.IngestionValidationErrors, objectId string, name string, err error) {
	if _, ok := errorsContainer[objectId]; !ok {
		errorsContainer[objectId] = make(map[string]string)
	}
	errorsContainer[objectId][name] = err.Error()
}

func (p *Parser) ParsePayload(ctx context.Context, table models.Table, json []byte) (models.ClientObject, error) {
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
		if field.Archived {
			continue
		}

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
			// Enrich ingested data for fields supporting it
			switch field.DataType {
			case models.IpAddress:
				switch ipBytes, ok := val.(net.IP); ok {
				case false:
					addError(allErrors, objectId, name, fmt.Errorf("expected IP address"))
				case true:
					switch ip, ok := netip.AddrFromSlice(ipBytes); ok {
					case false:
						addError(allErrors, objectId, name, fmt.Errorf("expected IP address"))
					case true:
						out[name] = ip.Unmap()

						if metadata := p.enricher.EnrichIp(ip.Unmap()); metadata != nil {
							key := fmt.Sprintf("%s.metadata", field.Name)
							if p.columnEscape {
								key = fmt.Sprintf(`"%s"`, key)
							}

							out[key] = metadata
						}
					}
				}

			case models.Coords:
				out[name] = val

				point := val.(models.Location)

				if metadata := p.enricher.EnrichCoordinates(point.X(), point.Y()); metadata != nil {
					key := fmt.Sprintf("%s.metadata", field.Name)
					if p.columnEscape {
						key = fmt.Sprintf(`"%s"`, key)
					}

					out[key] = map[string]string{
						"country": metadata.CountryCode2,
					}
				}

			default:
				out[name] = val
			}
		}
	}

	extraFields := make([]string, 0)

	for field := range result.Map() {
		if _, ok := table.Fields[field]; !ok {
			extraFields = append(extraFields, field)

			if p.disallowUnknownFields {
				addError(allErrors, objectId, field, errUnknownField)
			}
		}
	}

	if !p.disallowUnknownFields && len(extraFields) > 0 {
		utils.LoggerFromContext(ctx).WarnContext(ctx, "object was ingested with extra unknown fields",
			"object_type", table.Name,
			"object_id", objectId,
			"fields", extraFields)
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
	allowPatch            bool
	disallowUnknownFields bool
	columnEscape          bool
	enricher              PayloadEnrichementUsecase
}

type ParserOpt func(*parserOpts)

func WithAllowPatch() ParserOpt {
	return func(o *parserOpts) {
		o.allowPatch = true
	}
}

func WithAllowedPatch(allowed bool) ParserOpt {
	return func(o *parserOpts) {
		o.allowPatch = allowed
	}
}

func DisallowUnknownFields() ParserOpt {
	return func(o *parserOpts) {
		o.disallowUnknownFields = true
	}
}

func WithEnricher(uc PayloadEnrichementUsecase) ParserOpt {
	return func(o *parserOpts) {
		o.enricher = uc
	}
}

func WithColumnEscape() ParserOpt {
	return func(o *parserOpts) {
		o.columnEscape = true
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
			if t, err := time.Parse("2006-01-02T15:04:05", result.String()); err == nil {
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
		models.IpAddress: func(result gjson.Result) (any, error) {
			if result.Type != gjson.String {
				return nil, errIsInvalidString
			}
			ip := net.ParseIP(result.String())
			if ip == nil {
				return nil, fmt.Errorf("%w: cannot parse IP address", errIsInvalidIpAddress)
			}
			return ip, nil
		},
		models.Coords: func(result gjson.Result) (any, error) {
			if result.Type != gjson.String {
				return nil, errIsInvalidString
			}
			latS, lngS, ok := strings.Cut(result.String(), ",")
			if !ok {
				return nil, errIsInvalidCoords
			}
			lat, errLat := strconv.ParseFloat(latS, 64)
			lng, errLng := strconv.ParseFloat(lngS, 64)
			if errLat != nil || errLng != nil {
				return nil, errIsInvalidCoords
			}
			return models.Location{Geom: geos.NewPoint([]float64{lng, lat}).SetSRID(4326)}, nil
		},
	}

	options := parserOpts{}
	for _, opt := range opts {
		opt(&options)
	}

	return &Parser{
		parsers:               parsers,
		allowPatch:            options.allowPatch,
		disallowUnknownFields: options.disallowUnknownFields,
		columnEscape:          options.columnEscape,
		enricher:              options.enricher,
	}
}
