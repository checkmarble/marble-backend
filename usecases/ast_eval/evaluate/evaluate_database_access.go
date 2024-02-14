package evaluate

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type DatabaseAccess struct {
	OrganizationId              string
	DataModel                   models.DataModel
	Payload                     models.PayloadReader
	ClientSchemaExecutorFactory executor_factory.ClientSchemaExecutorFactory
	IngestedDataReadRepository  repositories.IngestedDataReadRepository
	ReturnFakeValue             bool
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
		return MakeEvaluateError(errors.Wrap(ast.ErrArgumentInvalidType, "path is not a slice of any in Evaluate DatabaseAccess"))
	}
	for _, v := range path {
		str, ok := v.(string)
		if !ok {
			return MakeEvaluateError(errors.Wrap(ast.ErrArgumentMustBeString, "path value is not a string in Evaluate DatabaseAccess"))
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
		return MakeEvaluateError(errors.Wrap(models.NullFieldReadError, fmt.Sprintf("value is null for object_id %s, in %s", objectId, errorMsg)))
	}

	return fieldValue, nil
}

func (d DatabaseAccess) getDbField(ctx context.Context, tableName models.TableName, fieldName models.FieldName, path []string) (interface{}, error) {

	if d.ReturnFakeValue {
		return DryRunGetDbField(d.DataModel, tableName, path, fieldName)
	}

	if db, err := d.ClientSchemaExecutorFactory.NewClientDbExecutor(ctx, d.OrganizationId); err != nil {
		return nil, err
	} else {
		return d.IngestedDataReadRepository.GetDbField(ctx, db, models.DbFieldReadParams{
			TriggerTableName: models.TableName(tableName),
			Path:             models.ToLinkNames(path),
			FieldName:        models.FieldName(fieldName),
			DataModel:        d.DataModel,
			Payload:          d.Payload,
		})
	}
}
