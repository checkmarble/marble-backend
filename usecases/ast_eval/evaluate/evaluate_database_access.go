package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/org_transaction"
)

type DatabaseAccess struct {
	OrganizationId             string
	DataModel                  models.DataModel
	Payload                    models.PayloadReader
	OrgTransactionFactory      org_transaction.Factory
	IngestedDataReadRepository repositories.IngestedDataReadRepository
	ReturnFakeValue            bool
}

func (d DatabaseAccess) Evaluate(arguments ast.Arguments) (any, error) {
	var pathStringArr []string
	tableNameStr, ok := (arguments.NamedArgs["tableName"].(string))
	if !ok {
		return nil, fmt.Errorf("tableName is not a string %w", ErrRuntimeExpression)
	}
	tableName := models.TableName(tableNameStr)

	fieldNameStr, ok := arguments.NamedArgs["fieldName"].(string)
	if !ok {
		return nil, fmt.Errorf("fieldName is not a string %w", ErrRuntimeExpression)
	}
	fieldName := models.FieldName(fieldNameStr)

	path, ok := arguments.NamedArgs["path"].([]any)
	if !ok {
		return nil, fmt.Errorf("path is not a string %w", ErrRuntimeExpression)
	}
	for _, v := range path {
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("path value is not a string %w", ErrRuntimeExpression)
		}
		pathStringArr = append(pathStringArr, str)
	}

	fieldValue, err := d.getDbField(tableName, fieldName, pathStringArr)

	if err != nil {
		errorMsg := fmt.Sprintf("tableName: %s, fieldName: %s, path: %v", tableName, fieldName, path)
		return nil, fmt.Errorf("DatabaseAccess: value not found: %s %w %w", errorMsg, err, ErrRuntimeExpression)
	}
	return fieldValue, nil
}

func (d DatabaseAccess) getDbField(tableName models.TableName, fieldName models.FieldName, path []string) (interface{}, error) {

	if d.ReturnFakeValue {
		return DryRunGetDbField(d.DataModel, tableName, path, fieldName)
	}

	return org_transaction.InOrganizationSchema(
		d.OrgTransactionFactory,
		d.OrganizationId,
		func(tx repositories.Transaction) (interface{}, error) {
			return d.IngestedDataReadRepository.GetDbField(tx, models.DbFieldReadParams{
				TriggerTableName: models.TableName(tableName),
				Path:             models.ToLinkNames(path),
				FieldName:        models.FieldName(fieldName),
				DataModel:        d.DataModel,
				Payload:          d.Payload,
			})
		})
}
