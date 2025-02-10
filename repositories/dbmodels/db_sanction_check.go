package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECKS = "sanction_checks"

var (
	SelectSanctionChecksColumn            = utils.ColumnList[DBSanctionCheck]()
	SelectSanctionChecksWithMatchesColumn = utils.ColumnList[DBSanctionCheckWithMatches]()
)

type DBSanctionCheck struct {
	Id                  string          `db:"id"`
	DecisionId          string          `db:"decision_id"`
	Status              string          `db:"status"`
	SearchInput         json.RawMessage `db:"search_input"`
	SearchDatasets      []string        `db:"search_datasets"`
	MatchThreshold      int             `db:"match_threshold"`
	MatchLimit          int             `db:"match_limit"`
	IsManual            bool            `db:"is_manual"`
	RequestedBy         *string         `db:"requested_by"`
	IsPartial           bool            `db:"is_partial"`
	IsArchived          bool            `db:"is_archived"`
	InitialHasMatches   bool            `db:"initial_has_matches"`
	WhitelistedEntities []string        `db:"whitelisted_entities"`
	CreatedAt           time.Time       `db:"created_at"`
	UpdatedAt           time.Time       `db:"updated_at"`
}

type DBSanctionCheckWithMatches struct {
	DBSanctionCheck
	Matches []DBSanctionCheckMatch `db:"matches"`
}

func AdaptSanctionCheck(dto DBSanctionCheck) (models.SanctionCheck, error) {
	cfg := models.OrganizationOpenSanctionsConfig{
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
	}

	return models.SanctionCheck{
		Id:                  dto.Id,
		DecisionId:          dto.DecisionId,
		Datasets:            dto.SearchDatasets,
		SearchInput:         dto.SearchInput,
		OrgConfig:           cfg,
		Partial:             dto.IsPartial,
		Status:              models.SanctionCheckStatusFrom(dto.Status),
		IsManual:            dto.IsManual,
		IsArchived:          dto.IsArchived,
		InitialHasMatches:   dto.InitialHasMatches,
		WhitelistedEntities: dto.WhitelistedEntities,
		RequestedBy:         dto.RequestedBy,
		CreatedAt:           dto.CreatedAt,
		UpdatedAt:           dto.UpdatedAt,
	}, nil
}

func AdaptSanctionCheckWithMatches(dto DBSanctionCheckWithMatches) (models.SanctionCheckWithMatches, error) {
	matches := make([]models.SanctionCheckMatch, 0, len(dto.Matches))
	for _, match := range dto.Matches {
		m, err := AdaptSanctionCheckMatch(match)
		if err != nil {
			return models.SanctionCheckWithMatches{}, err
		}

		matches = append(matches, m)
	}

	sc, _ := AdaptSanctionCheck(dto.DBSanctionCheck)
	return models.SanctionCheckWithMatches{
		SanctionCheck: sc,
		Matches:       matches,
		Count:         len(matches),
	}, nil
}
