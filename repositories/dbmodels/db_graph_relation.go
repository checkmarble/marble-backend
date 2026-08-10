package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_GRAPH_RELATIONS = "graph_relations"

var SelectGraphRelationColumn = utils.ColumnList[DBGraphRelation]()

type DBGraphRelation struct {
	Id         uuid.UUID `db:"id"`
	OrgId      uuid.UUID `db:"org_id"`
	Label      string    `db:"label"`
	LeftType   string    `db:"left_type"`
	LeftField  string    `db:"left_field"`
	RightType  string    `db:"right_type"`
	RightField string    `db:"right_field"`
	CreatedAt  time.Time `db:"created_at"`
}

func AdaptGraphRelation(db DBGraphRelation) (models.GraphRelation, error) {
	return models.GraphRelation{
		Id:         db.Id,
		OrgId:      db.OrgId,
		Label:      db.Label,
		LeftType:   db.LeftType,
		LeftField:  db.LeftField,
		RightType:  db.RightType,
		RightField: db.RightField,
		CreatedAt:  db.CreatedAt,
	}, nil
}
