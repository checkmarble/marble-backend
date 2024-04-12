package evaluate

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type DatabaseAccess struct {
	OrganizationId             string
	DataModel                  models.DataModel
	ClientObject               models.ClientObject
	ExecutorFactory            executor_factory.ExecutorFactory
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
		return MakeEvaluateError(errors.Wrap(ast.ErrArgumentInvalidType,
			"path is not a slice of any in Evaluate DatabaseAccess"))
	}
	for _, v := range path {
		str, ok := v.(string)
		if !ok {
			return MakeEvaluateError(errors.Wrap(ast.ErrArgumentMustBeString,
				"path value is not a string in Evaluate DatabaseAccess"))
		}
		pathStringArr = append(pathStringArr, str)
	}

	fieldValue, err := d.getDbField(ctx, tableName, fieldName, pathStringArr)
	if err != nil {
		errorMsg := fmt.Sprintf("Error reading value in DatabaseAccess: tableName %s, fieldName %s, path %v", tableName, fieldName, path)
		return MakeEvaluateError(errors.Join(
			errors.Wrap(ast.ErrDatabaseAccessNotFound, errorMsg),
			err,
		))
	}

	if fieldValue == nil {
		errorMsg := fmt.Sprintf("tableName: %s, fieldName: %s, path: %v", tableName, fieldName, path)
		objectId, _ := d.getDbField(ctx, tableName, "object_id", pathStringArr)
		return MakeEvaluateError(errors.Wrap(ast.ErrNullFieldRead,
			fmt.Sprintf("value is null for object_id %s, in %s", objectId, errorMsg)))
	}

	fieldValueStr, ok := fieldValue.(string)
	if ok {
		return pure_utils.Normalize(fieldValueStr), nil
	}

	return fieldValue, nil
}

func (d DatabaseAccess) getDbField(ctx context.Context, tableName models.TableName,
	fieldName models.FieldName, path []string,
) (interface{}, error) {
	if d.ReturnFakeValue {
		return DryRunGetDbField(d.DataModel, tableName, path, fieldName)
	}

	db, err := d.ExecutorFactory.NewClientDbExecutor(ctx, d.OrganizationId)
	if err != nil {
		return nil, err
	}
	return d.IngestedDataReadRepository.GetDbField(ctx, db, models.DbFieldReadParams{
		TriggerTableName: models.TableName(tableName),
		Path:             path,
		FieldName:        models.FieldName(fieldName),
		DataModel:        d.DataModel,
		ClientObject:     d.ClientObject,
	})
}
