package dbmodels

import (
	"math"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type DBCustomListResult struct {
	Id          string     `db:"id"`
	OrgId       uuid.UUID  `db:"organization_id"`
	Name        string     `db:"name"`
	Description string     `db:"description"`
	Kind        string     `db:"kind"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
	NumberItems int        `db:"nb_items"`
}

const TABLE_CUSTOM_LIST = "custom_lists"

var ColumnsSelectCustomList = utils.ColumnList[DBCustomListResult]()

func AdaptCustomList(db DBCustomListResult) (models.CustomList, error) {
	customList := models.CustomList{
		Id:             db.Id,
		OrganizationId: db.OrgId,
		Name:           db.Name,
		Description:    db.Description,
		CreatedAt:      db.CreatedAt,
		Kind:           models.CustomListKindFromString(db.Kind),
		UpdatedAt:      db.UpdatedAt,
		DeletedAt:      db.DeletedAt,
	}

	if customList.Kind == models.CustomListUnknown {
		return models.CustomList{}, errors.Newf("unknown custom list kind %s", db.Kind)
	}

	customList.ValuesCount = &models.ValuesInfo{
		Count:   int(math.Min(float64(db.NumberItems), float64(models.VALUES_COUNT_LIMIT))),
		HasMore: db.NumberItems > models.VALUES_COUNT_LIMIT,
	}

	return customList, nil
}
