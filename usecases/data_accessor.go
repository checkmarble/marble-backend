package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DataAccessor struct {
	DataModel                  models.DataModel
	Payload                    models.PayloadReader
	orgTransactionFactory      organization.OrgTransactionFactory
	organizationId             string
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	customListRepository         repositories.CustomListRepository
}

func (d *DataAccessor) GetPayloadField(fieldName string) (interface{}, error) {
	return d.Payload.ReadFieldFromPayload(models.FieldName(fieldName))
}

func (d *DataAccessor) GetDbField(ctx context.Context, triggerTableName string, path []string, fieldName string) (interface{}, error) {

	return organization.TransactionInOrgSchemaReturnValue(
		d.orgTransactionFactory,
		d.organizationId,
		func(tx repositories.Transaction) (interface{}, error) {
			return d.ingestedDataReadRepository.GetDbField(tx, models.DbFieldReadParams{
				TriggerTableName: models.TableName(triggerTableName),
				Path:             models.ToLinkNames(path),
				FieldName:        models.FieldName(fieldName),
				DataModel:        d.DataModel,
				Payload:          d.Payload,
			})
		})
}

func (d *DataAccessor) GetDbHandle() (db *pgxpool.Pool, schema string, err error) {

	databaseShema, err := d.orgTransactionFactory.OrganizationDatabaseSchema(d.organizationId)
	if err != nil {
		return nil, "", err
	}

	pool, err := d.orgTransactionFactory.OrganizationDbPool(databaseShema)
	if err != nil {
		return nil, "", err
	}

	return pool, databaseShema.Schema, nil
}

func (d *DataAccessor) GetDbCustomListValues(ctx context.Context, customListId string) (customListValues []string, err error) {
	values, err := d.customListRepository.GetCustomListValues(nil, models.GetCustomListValuesInput{
		Id:    customListId,
		OrgId: d.organizationId,
	})
	if err != nil {
		return
	}
	for _, value := range values {
		customListValues = append(customListValues, value.Value)
	}
	return
}
