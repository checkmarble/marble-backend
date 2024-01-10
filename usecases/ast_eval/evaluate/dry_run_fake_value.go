package evaluate

import (
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

func DryRunPayload(table models.Table) map[string]any {

	result := make(map[string]any)
	for fieldName, field := range table.Fields {
		fullFieldName := fmt.Sprintf("%s.%s", table.Name, fieldName)
		result[string(fieldName)] = DryRunValue("Payload", fullFieldName, field)
	}

	return result
}

func DryRunGetDbField(dataModel models.DataModel, triggerTableName models.TableName, path []string, fieldName models.FieldName) (any, error) {

	table, ok := dataModel.Tables[triggerTableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in data model", triggerTableName)
	}

	for _, linkName := range path {
		link, ok := table.LinksToSingle[models.LinkName(linkName)]
		if !ok {
			return nil, errors.New(fmt.Sprintf("link %s not found in table %s", linkName, table.Name))
		}

		table, ok = dataModel.Tables[link.LinkedTableName]
		if !ok {
			return nil, errors.New(fmt.Sprintf("table %s not found in data model", triggerTableName))
		}
	}

	field, ok := table.Fields[fieldName]
	if !ok {
		return nil, errors.New(fmt.Sprintf("field %s not found in table %s", fieldName, table.Name))
	}

	fullFieldName := fmt.Sprintf("%s.%s.%s", triggerTableName, strings.Join(path, "."), fieldName)
	return DryRunValue("DbAccess", fullFieldName, field), nil
}

func DryRunValue(prefix string, fieldName string, field models.Field) any {
	switch field.DataType {
	case models.Bool:
		return true
	case models.String:
		return fmt.Sprintf("fake value for %s:%s", prefix, fieldName)
	case models.Int:
		return 1
	case models.Float:
		return 1.0
	case models.Timestamp:
		return time.Now()
	default:
		return nil
	}
}

func DryRunQueryAggregatedValue(datamodel models.DataModel, tableName models.TableName, fieldName models.FieldName, aggregator ast.Aggregator) (any, error) {
	table, ok := datamodel.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in data model", tableName)
	}

	field, ok := table.Fields[fieldName]
	if !ok {
		return nil, fmt.Errorf("field %s not found in table %s", fieldName, table.Name)
	}

	switch aggregator {
	case ast.AGGREGATOR_COUNT, ast.AGGREGATOR_COUNT_DISTINCT:
		return 10, nil
	case ast.AGGREGATOR_SUM, ast.AGGREGATOR_AVG, ast.AGGREGATOR_MAX, ast.AGGREGATOR_MIN:
		return DryRunValue("Aggregator", fmt.Sprintf("%s.%s", tableName, fieldName), field), nil
	default:
		return nil, errors.New(fmt.Sprintf("aggregator %s not supported", aggregator))
	}
}
