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

func (d DatabaseAccess) Evaluate(arguments ast.Arguments) (any, []error) {

	tableNameStr, tableNameErr := AdaptNamedArgument(arguments.NamedArgs, "tableName", adaptArgumentToString)
	fieldNameStr, fieldNameErr := AdaptNamedArgument(arguments.NamedArgs, "fieldName", adaptArgumentToString)

	errs := filterNilErrors(tableNameErr, fieldNameErr)
	if len(errs) > 0 {
		return nil, errs
	}

	var pathStringArr []string
	tableName := models.TableName(tableNameStr)
	fieldName := models.FieldName(fieldNameStr)

	path, ok := arguments.NamedArgs["path"].([]any)
	if !ok {
		return MakeEvaluateError(fmt.Errorf("path is not a string"))
	}
	for _, v := range path {
		str, ok := v.(string)
		if !ok {
			return MakeEvaluateError(fmt.Errorf("path value is not a string"))
		}
		pathStringArr = append(pathStringArr, str)
	}

	fieldValue, err := d.getDbField(tableName, fieldName, pathStringArr)

	if err != nil {
		errorMsg := fmt.Sprintf("tableName: %s, fieldName: %s, path: %v", tableName, fieldName, path)
		return MakeEvaluateError(fmt.Errorf("DatabaseAccess: value not found: %s %w", errorMsg, err))
	}

	if fieldValue == nil {
		errorMsg := fmt.Sprintf("tableName: %s, fieldName: %s, path: %v", tableName, fieldName, path)
		objectId, _ := d.getDbField(tableName, "object_id", pathStringArr)
		return MakeEvaluateError(fmt.Errorf("value is null for object_id %s, in %s %w", objectId, errorMsg, models.NullFieldReadError))
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
