package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const TABLE_SAR_DECLARATION_SELECTIONS = "sar_declaration_selections"

type DBSarSelectedObject struct {
	Id            uuid.UUID       `db:"id"`
	DeclarationId uuid.UUID       `db:"declaration_id"`
	TableName     string          `db:"table_name"`
	ObjectId      string          `db:"object_id"`
	Snapshot      json.RawMessage `db:"snapshot"`
	Origin        json.RawMessage `db:"origin"`
	ReachedFrom   json.RawMessage `db:"reached_from"`
	CreatedAt     time.Time       `db:"created_at"`
}

var SarSelectionColumns = utils.ColumnList[DBSarSelectedObject]()

func AdaptSarSelectedObject(db DBSarSelectedObject) (models.SarSelectedObject, error) {
	out := models.SarSelectedObject{
		Id:            db.Id,
		DeclarationId: db.DeclarationId,
		TableName:     db.TableName,
		ObjectId:      db.ObjectId,
		CreatedAt:     db.CreatedAt,
	}
	if err := json.Unmarshal(db.Snapshot, &out.Snapshot); err != nil {
		return models.SarSelectedObject{}, errors.Wrap(err, "invalid sar selection snapshot")
	}
	if err := json.Unmarshal(db.Origin, &out.Origin); err != nil {
		return models.SarSelectedObject{}, errors.Wrap(err, "invalid sar selection origin")
	}
	if len(db.ReachedFrom) > 0 && string(db.ReachedFrom) != "null" {
		var rf models.SarReachedFrom
		if err := json.Unmarshal(db.ReachedFrom, &rf); err != nil {
			return models.SarSelectedObject{}, errors.Wrap(err, "invalid sar selection reached_from")
		}
		out.ReachedFrom = &rf
	}
	return out, nil
}
