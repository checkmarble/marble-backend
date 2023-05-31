package app

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DataAccessorImpl struct {
	DataModel  models.DataModel
	Payload    models.Payload
	repository RepositoryInterface
}

func (d *DataAccessorImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return d.Payload.ReadFieldFromPayload(models.FieldName(fieldName))
}
func (d *DataAccessorImpl) GetDbField(triggerTableName string, path []string, fieldName string) (interface{}, error) {
	return d.repository.GetDbField(context.TODO(), models.DbFieldReadParams{
		TriggerTableName: models.TableName(triggerTableName),
		Path:             models.ToLinkNames(path),
		FieldName:        models.FieldName(fieldName),
		DataModel:        d.DataModel,
		Payload:          d.Payload,
	})
}
func (d *DataAccessorImpl) GetDbHandle() *pgxpool.Pool {
	return d.repository.GetDbPool()
}

func (d *DataAccessorImpl) GetVariable(ctx context.Context, id string) (models.Variable, error) {
	_, err := utils.OrgIDFromCtx(ctx, nil)
	if err != nil {
		return models.Variable{}, err
	}

	// Eventually, do this
	// return d.repository.GetVariable(ctx, orgID, id)
	// and check here if the orgId is correct
	return models.Variable{
		Name:         "Julaya test variable",
		ArgumentType: models.String,
		OutputType:   models.Float,
		SqlTemplate: `SELECT SUM(amount)
		FROM transactions
		WHERE account_id = $1
			AND type='CASH')
			AND status='VALIDATED'
			AND direction='PAYOUT'
			AND transaction_at > NOW() - INTERVAL '1 MONTH'
			AND transaction_at < NOW()`,
	}, nil
}
