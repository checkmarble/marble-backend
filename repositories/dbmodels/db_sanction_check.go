package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECKS = "sanction_checks"

var SelectSanctionChecksColumn = utils.ColumnList[DBSanctionCheck]()

type DBSanctionCheck struct {
	Id              string                 `db:"id"`
	DecisionId      string                 `db:"decision_id"`
	Status          string                 `db:"status"`
	SearchInput     json.RawMessage        `db:"search_input"`
	SearchDatasets  []string               `db:"search_datasets"`
	SearchThreshold *int                   `db:"search_threshold"`
	IsManual        bool                   `db:"is_manual"`
	RequestedBy     *string                `db:"requested_by"`
	IsPartial       bool                   `db:"is_partial"`
	IsArchived      bool                   `db:"is_archived"`
	CreatedAt       time.Time              `db:"created_at"`
	UpdatedAt       time.Time              `db:"updated_at"`
	Matches         []DBSanctionCheckMatch `db:"matches"`
}

func AdaptSanctionCheck(dto DBSanctionCheck) (models.SanctionCheck, error) {
	cfg := models.OrganizationOpenSanctionsConfig{
		MatchThreshold: dto.SearchThreshold,
	}

	matches := make([]models.SanctionCheckMatch, 0, len(dto.Matches))
	for _, match := range dto.Matches {
		m, err := AdaptSanctionCheckMatch(match)
		if err != nil {
			return models.SanctionCheck{}, err
		}

		matches = append(matches, m)
	}

	return models.SanctionCheck{
		Id:          dto.Id,
		DecisionId:  dto.DecisionId,
		Datasets:    dto.SearchDatasets,
		Query:       dto.SearchInput,
		OrgConfig:   cfg,
		Partial:     dto.IsPartial,
		Status:      models.SanctionCheckStatusFrom(dto.Status),
		IsManual:    dto.IsManual,
		IsArchived:  dto.IsArchived,
		RequestedBy: dto.RequestedBy,
		CreatedAt:   dto.CreatedAt,
		UpdatedAt:   dto.UpdatedAt,
		Matches:     matches,
		Count:       len(matches),
	}, nil
}
