package evaluate

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type DatabaseAccess struct {
	OrganizationId             string
	DataModel                  models.DataModel
	Payload                    models.PayloadReader
	OrgTransactionFactory      transaction.Factory
	IngestedDataReadRepository repositories.IngestedDataReadRepository
	ReturnFakeValue            bool
}

func (d DatabaseAccess) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {

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
		return MakeEvaluateError(fmt.Errorf("path is not a string %w", ast.ErrArgumentMustBeString))
	}
	for _, v := range path {
		str, ok := v.(string)
		if !ok {
			return MakeEvaluateError(fmt.Errorf("path value is not a string %w", ast.ErrArgumentMustBeString))
		}
		pathStringArr = append(pathStringArr, str)
	}

	fieldValue, err := d.getDbField(ctx, tableName, fieldName, pathStringArr)

	if err != nil {
		errorMsg := fmt.Sprintf("tableName: %s, fieldName: %s, path: %v", tableName, fieldName, path)
		return MakeEvaluateError(fmt.Errorf("DatabaseAccess: value not found: %s %w %w", errorMsg, err, ast.ErrDatabaseAccessNotFound))
	}

	if fieldValue == nil {
		errorMsg := fmt.Sprintf("tableName: %s, fieldName: %s, path: %v", tableName, fieldName, path)
		objectId, _ := d.getDbField(ctx, tableName, "object_id", pathStringArr)
		return MakeEvaluateError(fmt.Errorf("value is null for object_id %s, in %s %w", objectId, errorMsg, models.NullFieldReadError))
	}

	return fieldValue, nil
}

func (d DatabaseAccess) getDbField(ctx context.Context, tableName models.TableName, fieldName models.FieldName, path []string) (interface{}, error) {

	if d.ReturnFakeValue {
		return DryRunGetDbField(d.DataModel, tableName, path, fieldName)
	}

	return transaction.InOrganizationSchema(
		ctx,
		d.OrgTransactionFactory,
		d.OrganizationId,
		func(tx repositories.Transaction) (interface{}, error) {
			return d.IngestedDataReadRepository.GetDbField(ctx, tx, models.DbFieldReadParams{
				TriggerTableName: models.TableName(tableName),
				Path:             models.ToLinkNames(path),
				FieldName:        models.FieldName(fieldName),
				DataModel:        d.DataModel,
				Payload:          d.Payload,
			})
		})
}
