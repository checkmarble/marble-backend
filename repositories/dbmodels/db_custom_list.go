package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBCustomListResult struct {
	Id          string     `db:"id"`
	OrgId       string     `db:"organization_id"`
	Name        string     `db:"name"`
	Description string     `db:"description"`
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
		UpdatedAt:      db.UpdatedAt,
		DeletedAt:      db.DeletedAt,
	}

	customList.ValuesCount = &models.ValuesInfo{
		Count:   db.NumberItems,
		HasMore: db.NumberItems > models.VALUES_COUNT_LIMIT,
	}

	return customList, nil
}
