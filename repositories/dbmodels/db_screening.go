package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_SCREENINGS = "screenings"

var (
	SelectScreeningColumn            = utils.ColumnList[DBScreening]()
	SelectScreeningAndConfigColumn   = utils.ColumnList[DBScreeningAndConfig]()
	SelectScreeningBaseInfoColumn    = utils.ColumnList[DBScreeningBaseInfo]()
	SelectScreeningWithMatchesColumn = utils.ColumnList[DBScreeningWithMatches]()
)

type DBScreening struct {
	Id                  string                           `db:"id"`
	DecisionId          string                           `db:"decision_id"`
	OrgId               uuid.UUID                        `db:"org_id"`
	ScreeningConfigId   string                           `db:"screening_config_id"`
	Status              string                           `db:"status"`
	SearchInput         json.RawMessage                  `db:"search_input"`
	InitialQuery        []models.OpenSanctionsCheckQuery `db:"initial_query"`
	CounterpartyId      *string                          `db:"counterparty_id"`
	MatchThreshold      int                              `db:"match_threshold"`
	MatchLimit          int                              `db:"match_limit"`
	IsManual            bool                             `db:"is_manual"`
	RequestedBy         *string                          `db:"requested_by"`
	IsPartial           bool                             `db:"is_partial"`
	IsArchived          bool                             `db:"is_archived"`
	InitialHasMatches   bool                             `db:"initial_has_matches"`
	ErrorCodes          []string                         `db:"error_codes"`
	NumberOfMatches     *int                             `db:"number_of_matches"`
	CreatedAt           time.Time                        `db:"created_at"`
	UpdatedAt           time.Time                        `db:"updated_at"`
}

type DBScreeningAndConfig struct {
	DBScreening

	// Fields from config (joined via screening_config_id)
	ConfigId string   `db:"config_id"`
	StableId string   `db:"stable_id"`
	Name     string   `db:"name"`
	Datasets []string `db:"datasets"`
}

type DBScreeningWithMatches struct {
	DBScreeningAndConfig
	Matches []DBScreeningMatch `db:"matches"`
}

func adaptScreeningWithoutConfig(dto DBScreening) (models.Screening, error) {
	cfg := models.OrganizationOpenSanctionsConfig{
		MatchThreshold: dto.MatchThreshold,
		MatchLimit:     dto.MatchLimit,
	}
	numberOfMatches := 0
	if dto.NumberOfMatches != nil {
		numberOfMatches = *dto.NumberOfMatches
	}

	return models.Screening{
		Id:                           dto.Id,
		DecisionId:                   dto.DecisionId,
		OrgId:                        dto.OrgId,
		ScreeningConfigId:            dto.ScreeningConfigId,
		UniqueCounterpartyIdentifier: dto.CounterpartyId,
		SearchInput:                  dto.SearchInput,
		InitialQuery:                 dto.InitialQuery,
		OrgConfig:                    cfg,
		Partial:                      dto.IsPartial,
		Status:                       models.ScreeningStatusFrom(dto.Status),
		IsManual:                     dto.IsManual,
		IsArchived:                   dto.IsArchived,
		InitialHasMatches:            dto.InitialHasMatches,
		RequestedBy:                  dto.RequestedBy,
		ErrorCodes:                   dto.ErrorCodes,
		NumberOfMatches:              numberOfMatches,
		CreatedAt:                    dto.CreatedAt,
		UpdatedAt:                    dto.UpdatedAt,
	}, nil
}

func AdaptScreening(dto DBScreeningAndConfig) (models.Screening, error) {
	sc, _ := adaptScreeningWithoutConfig(dto.DBScreening)
	sc.Config = models.ScreeningConfigRef{
		Id:       dto.ConfigId,
		StableId: dto.StableId,
		Name:     dto.Name,
		Datasets: dto.Datasets,
	}
	return sc, nil
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

	sc, _ := AdaptScreening(dto.DBScreeningAndConfig)
	return models.ScreeningWithMatches{
		Screening: sc,
		Matches:   matches,
	}, nil
}

// Screening with base information: no reading of matches
type DBScreeningBaseInfo struct {
	Id              string    `db:"id"`
	DecisionId      string    `db:"decision_id"`
	OrgId           uuid.UUID `db:"org_id"`
	Status          string    `db:"status"`
	RequestedBy     *string   `db:"requested_by"`
	IsPartial       bool      `db:"is_partial"`
	NumberOfMatches *int      `db:"number_of_matches"`
	CreatedAt       time.Time `db:"created_at"`
}

type DBScreeningBaseInfoWithName struct {
	DBScreeningBaseInfo
	Name string `db:"name"` // field is on screening_configs table and requires a join
}

func AdaptScreeningBaseInfo(dto DBScreeningBaseInfoWithName) (models.ScreeningBaseInfo, error) {
	numberOfMatches := 0
	if dto.NumberOfMatches != nil {
		numberOfMatches = *dto.NumberOfMatches
	}
	return models.ScreeningBaseInfo{
		Id:              dto.Id,
		DecisionId:      dto.DecisionId,
		OrgId:           dto.OrgId,
		Status:          models.ScreeningStatusFrom(dto.Status),
		RequestedBy:     dto.RequestedBy,
		Partial:         dto.IsPartial,
		Name:            dto.Name,
		NumberOfMatches: numberOfMatches,
		CreatedAt:       dto.CreatedAt,
	}, nil
}
