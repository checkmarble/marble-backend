package evaluate

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"
)

type DatabaseAccess struct {
	OrganizationIdOfContext    string
	DataModelRepository        repositories.DataModelRepository
	Payload                    models.PayloadReader
	OrgTransactionFactory      organization.OrgTransactionFactory
	IngestedDataReadRepository repositories.IngestedDataReadRepository
}

func NewDatabaseAccess(otf organization.OrgTransactionFactory, idrr repositories.IngestedDataReadRepository,
	dm repositories.DataModelRepository, payload models.PayloadReader,
	organizationIdOfContext string) DatabaseAccess {
	return DatabaseAccess{
		OrganizationIdOfContext:    organizationIdOfContext,
		DataModelRepository:        dm,
		Payload:                    payload,
		OrgTransactionFactory:      otf,
		IngestedDataReadRepository: idrr,
	}
}

func (d DatabaseAccess) Evaluate(arguments ast.Arguments) (any, error) {
	var pathStringArr []string
	tableName, ok := arguments.NamedArgs["tableName"].(string)
	if !ok {
		return nil, fmt.Errorf("tableName is not a string %w", ErrRuntimeExpression)
	}
	fieldName, ok := arguments.NamedArgs["fieldName"].(string)
	if !ok {
		return nil, fmt.Errorf("fieldName is not a string %w", ErrRuntimeExpression)
	}
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

func (d DatabaseAccess) getDbField(tableName string, fieldName string, path []string) (interface{}, error) {

	dm, err := d.DataModelRepository.GetDataModel(nil, d.OrganizationIdOfContext)
	if errors.Is(err, models.NotFoundInRepositoryError) {
		return models.Decision{}, fmt.Errorf("data model not found: %w", models.NotFoundError)
	} else if err != nil {
		return models.Decision{}, fmt.Errorf("error getting data model: %w", err)
	}

	return organization.TransactionInOrgSchemaReturnValue(
		d.OrgTransactionFactory,
		d.OrganizationIdOfContext,
		func(tx repositories.Transaction) (interface{}, error) {
			return d.IngestedDataReadRepository.GetDbField(tx, models.DbFieldReadParams{
				TriggerTableName: models.TableName(tableName),
				Path:             models.ToLinkNames(path),
				FieldName:        models.FieldName(fieldName),
				DataModel:        dm,
				Payload:          d.Payload,
			})
		})
}
