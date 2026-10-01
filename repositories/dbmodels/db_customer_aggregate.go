package dbmodels

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbCustomerAggregate struct {
	Id         uuid.UUID       `db:"id"`
	OrgId      uuid.UUID       `db:"org_id"`
	TableId    uuid.UUID       `db:"table_id"`
	Name       string          `db:"name"`
	Type       string          `db:"kind"`
	Expression json.RawMessage `db:"expression"`
	TimeSlice  string          `db:"time_slice"`
}

const TABLE_CUSTOMER_AGGREGATES = "customer_aggregates"

var SelectCustomerAggregatesColumns = utils.ColumnList[DbCustomerAggregate]()

func AdaptCustomerAggregate(db DbCustomerAggregate) (models.CustomerAggregate, error) {
	expr, err := AdaptSerializedAstExpression(db.Expression)
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	return models.CustomerAggregate{
		Id:         db.Id,
		OrgId:      db.OrgId,
		TableId:    db.TableId,
		Name:       db.Name,
		Type:       models.CustomerAggregateType(db.Type),
		Expression: *expr,
		TimeSlice:  db.TimeSlice,
	}, nil
}
