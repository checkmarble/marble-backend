package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbUserUnavailability struct {
	Id        uuid.UUID  `db:"id"`
	OrgId     uuid.UUID  `db:"id"`
	UserId    uuid.UUID  `db:"id"`
	FromDate  time.Time  `db:"from_date"`
	ToDate    time.Time  `db:"until_date"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

const TABLE_USER_UNAVAILABILITIES = "user_unavailabilities"

var ColumnsSelectUserUnavailabilities = utils.ColumnList[DbUserUnavailability]()
