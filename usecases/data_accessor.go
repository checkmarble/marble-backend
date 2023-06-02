package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DataAccessor struct {
	DataModel                  models.DataModel
	Payload                    models.Payload
	dbPoolRepository           repositories.DbPoolRepository
	ingestedDataReadRepository repositories.IngestedDataReadRepository
}

func (d *DataAccessor) GetPayloadField(fieldName string) (interface{}, error) {
	return d.Payload.ReadFieldFromPayload(models.FieldName(fieldName))
}
func (d *DataAccessor) GetDbField(ctx context.Context, triggerTableName string, path []string, fieldName string) (interface{}, error) {
	return d.ingestedDataReadRepository.GetDbField(ctx, models.DbFieldReadParams{
		TriggerTableName: models.TableName(triggerTableName),
		Path:             models.ToLinkNames(path),
		FieldName:        models.FieldName(fieldName),
		DataModel:        d.DataModel,
		Payload:          d.Payload,
	})
}
func (d *DataAccessor) GetDbHandle() *pgxpool.Pool {
	return d.dbPoolRepository.GetDbPool()
}
