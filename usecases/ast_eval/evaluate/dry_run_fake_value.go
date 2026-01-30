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
		result[fieldName] = DryRunValue("Payload", fullFieldName, field)
	}

	return result
}

func DryRunGetDbField(dataModel models.DataModel, triggerTableName string, path []string, fieldName string) (any, error) {
	table, ok := dataModel.Tables[triggerTableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in data model", triggerTableName)
	}

	for _, linkName := range path {
		link, ok := table.LinksToSingle[linkName]
		if !ok {
			return nil, errors.New(fmt.Sprintf("link %s not found in table %s", linkName, table.Name))
		}

		table, ok = dataModel.Tables[link.ParentTableName]
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

// DryRunListIngestedObjects validates that a NavigationOption configuration matches
// an existing navigation option in the data model and returns fake DataModelObjects.
// Returns the same type as ListIngestedObjects: ([]models.DataModelObject, error)
func DryRunListIngestedObjects(dataModel models.DataModel, nav ast.NavigationOption, fieldsToRead ...string) ([]models.DataModelObject, error) {
	// Get the source table (navigation options are stored on the source table)
	sourceTable, ok := dataModel.Tables[nav.SourceTableName]
	if !ok {
		return nil, fmt.Errorf("source table %s not found in data model", nav.SourceTableName)
	}

	// Check if a matching navigation option exists in the source table
	found := false
	for _, existingNav := range sourceTable.NavigationOptions {
		if existingNav.SourceTableName == nav.SourceTableName &&
			existingNav.SourceFieldName == nav.SourceFieldName &&
			existingNav.TargetTableName == nav.TargetTableName &&
			existingNav.FilterFieldName == nav.TargetFieldName {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("navigation option from %s.%s to %s.%s not found in data model",
			nav.SourceTableName, nav.SourceFieldName, nav.TargetTableName, nav.TargetFieldName)
	}

	// Get the target table to validate fields and generate fake values
	targetTable, ok := dataModel.Tables[nav.TargetTableName]
	if !ok {
		return nil, fmt.Errorf("target table %s not found in data model", nav.TargetTableName)
	}

	// Build fake data for the requested fields
	fakeData := make(map[string]any)
	fakeData["object_id"] = "fake_object_id_for_dry_run" // object_id is always included

	for _, fieldName := range fieldsToRead {
		if fieldName == "object_id" {
			continue // already added
		}
		field, ok := targetTable.Fields[fieldName]
		if !ok {
			return nil, fmt.Errorf("field %s not found in table %s", fieldName, nav.TargetTableName)
		}
		fullFieldName := fmt.Sprintf("%s.%s", nav.TargetTableName, fieldName)
		fakeData[fieldName] = DryRunValue("ListIngestedObjects", fullFieldName, field)
	}

	// Return fake DataModelObjects to simulate navigation results
	return []models.DataModelObject{
		{
			Data:     fakeData,
			Metadata: map[string]any{},
		},
	}, nil
}

func DryRunQueryAggregatedValue(datamodel models.DataModel, tableName string, fieldName string, aggregator ast.Aggregator) (any, error) {
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
	case ast.AGGREGATOR_SUM, ast.AGGREGATOR_AVG, ast.AGGREGATOR_MAX, ast.AGGREGATOR_MIN, ast.AGGREGATOR_STDDEV, ast.AGGREGATOR_PERCENTILE, ast.AGGREGATOR_MEDIAN:
		return DryRunValue("Aggregator", fmt.Sprintf("%s.%s", tableName, fieldName), field), nil
	default:
		return nil, errors.New(fmt.Sprintf("aggregator %s not supported", aggregator))
	}
}
