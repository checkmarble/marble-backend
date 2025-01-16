package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECKS = "sanction_checks"

var SelectSanctionChecksColumn = utils.ColumnList[DBSanctionCheck]()

type DBSanctionCheck struct {
	Id              string    `db:"id"`
	DecisionId      string    `db:"decision_id"`
	Status          string    `db:"status"`
	SearchInput     []byte    `db:"search_input"`
	SearchDatasets  []string  `db:"search_datasets"`
	SearchThreshold float64   `db:"search_threshold"`
	IsManual        bool      `db:"is_manual"`
	RequestedBy     string    `db:"requested_by"`
	IsArchived      bool      `db:"is_archived"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}
