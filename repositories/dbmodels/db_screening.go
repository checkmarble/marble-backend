package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SCREENINGS = "sanction_checks"

var (
	SelectScreeningColumn            = utils.ColumnList[DBScreening]()
	SelectScreeningWithMatchesColumn = utils.ColumnList[DBScreeningWithMatches]()
)

type DBScreening struct {
	Id                  string                           `db:"id"`
	DecisionId          string                           `db:"decision_id"`
	OrgId               string                           `db:"org_id"`
	ScreeningConfigId   string                           `db:"sanction_check_config_id"`
	Status              string                           `db:"status"`
	SearchInput         json.RawMessage                  `db:"search_input"`
	InitialQuery        []models.OpenSanctionsCheckQuery `db:"initial_query"`
	SearchDatasets      []string                         `db:"search_datasets"`
	MatchThreshold      int                              `db:"match_threshold"`
	MatchLimit          int                              `db:"match_limit"`
	IsManual            bool                             `db:"is_manual"`
	RequestedBy         *string                          `db:"requested_by"`
	IsPartial           bool                             `db:"is_partial"`
	IsArchived          bool                             `db:"is_archived"`
	InitialHasMatches   bool                             `db:"initial_has_matches"`
	WhitelistedEntities []string                         `db:"whitelisted_entities"`
	ErrorCodes          []string                         `db:"error_codes"`
	NumberOfMatches     *int                             `db:"number_of_matches"`
	CreatedAt           time.Time                        `db:"created_at"`
	UpdatedAt           time.Time                        `db:"updated_at"`
}

type DBScreeningWithMatches struct {
	DBScreening
	Matches []DBScreeningMatch `db:"matches"`
}

func AdaptScreening(dto DBScreening) (models.Screening, error) {
	cfg := models.OrganizationOpenSanctionsConfig{
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
	}
	numberOfMatches := 0
	if dto.NumberOfMatches != nil {
		numberOfMatches = *dto.NumberOfMatches
	}

	return models.Screening{
		Id:                  dto.Id,
		DecisionId:          dto.DecisionId,
		OrgId:               dto.OrgId,
		ScreeningConfigId:   dto.ScreeningConfigId,
		Datasets:            dto.SearchDatasets,
		SearchInput:         dto.SearchInput,
		InitialQuery:        dto.InitialQuery,
		OrgConfig:           cfg,
		Partial:             dto.IsPartial,
		Status:              models.ScreeningStatusFrom(dto.Status),
		IsManual:            dto.IsManual,
		IsArchived:          dto.IsArchived,
		InitialHasMatches:   dto.InitialHasMatches,
		WhitelistedEntities: dto.WhitelistedEntities,
		RequestedBy:         dto.RequestedBy,
		ErrorCodes:          dto.ErrorCodes,
		NumberOfMatches:     numberOfMatches,
		CreatedAt:           dto.CreatedAt,
		UpdatedAt:           dto.UpdatedAt,
	}, nil
}

func AdaptScreeningWithMatches(dto DBScreeningWithMatches) (models.ScreeningWithMatches, error) {
	matches := make([]models.ScreeningMatch, 0, len(dto.Matches))
	for _, match := range dto.Matches {
		m, err := AdaptScreeningMatch(match)
		if err != nil {
			return models.ScreeningWithMatches{}, err
		}

		matches = append(matches, m)
	}

	sc, _ := AdaptScreening(dto.DBScreening)
	return models.ScreeningWithMatches{
		Screening: sc,
		Matches:   matches,
	}, nil
}
