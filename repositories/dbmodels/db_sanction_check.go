package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECKS = "sanction_checks"

var SelectSanctionChecksColumn = utils.ColumnList[DBSanctionCheck]()

type DBSanctionCheck struct {
	Id              string                    `db:"id"`
	DecisionId      string                    `db:"decision_id"`
	Status          string                    `db:"status"`
	SearchInput     models.OpenSanctionsQuery `db:"search_input"`
	SearchDatasets  []string                  `db:"search_datasets"`
	SearchThreshold *int                      `db:"search_threshold"`
	IsManual        bool                      `db:"is_manual"`
	IsPartial       bool                      `db:"is_partial"`
	RequestedBy     *string                   `db:"requested_by"`
	IsArchived      bool                      `db:"is_archived"`
	CreatedAt       time.Time                 `db:"created_at"`
	UpdatedAt       time.Time                 `db:"updated_at"`
}

func AdaptSanctionCheck(dto DBSanctionCheck) (models.SanctionCheck, error) {
	cfg := models.OrganizationOpenSanctionsConfig{
		MatchThreshold: dto.SearchThreshold,
		Datasets:       dto.SearchDatasets,
	}

	query := models.OpenSanctionsQuery{
		Queries:   dto.SearchInput.Queries,
		OrgConfig: cfg,
	}

	return models.SanctionCheck{
		Id:          dto.Id,
		DecisionId:  dto.DecisionId,
		Query:       query,
		Partial:     dto.IsPartial,
		Status:      dto.Status,
		IsManual:    dto.IsManual,
		RequestedBy: dto.RequestedBy,
	}, nil
}
