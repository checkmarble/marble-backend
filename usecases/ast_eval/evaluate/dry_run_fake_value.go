package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"strings"
	"time"
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
			return nil, fmt.Errorf("link %s not found in table %s", linkName, table.Name)
		}

		table, ok = dataModel.Tables[link.LinkedTableName]
		if !ok {
			return nil, fmt.Errorf("table %s not found in data model", triggerTableName)
		}
	}

	field, ok := table.Fields[fieldName]
	if !ok {
		return nil, fmt.Errorf("field %s not found in table %s", fieldName, table.Name)
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
		t, _ := time.Parse(time.RFC3339, time.RFC3339)
		return t
	default:
		return nil
	}
}
