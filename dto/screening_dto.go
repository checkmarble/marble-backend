package dto

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
)

var (
	ValidScreeningStatuses      = []string{"in_review", "confirmed_hit", "error"}
	ValidScreeningMatchStatuses = []string{"pending", "confirmed_hit", "no_hit"}
)

type ScreeningDto struct {
	Id           string                           `json:"id"`
	Config       ScreeningConfigRefDto            `json:"config"`
	Status       string                           `json:"status"`
	Request      *ScreeningRequestDto             `json:"request"`
	InitialQuery []models.OpenSanctionsCheckQuery `json:"initial_query"`
	Partial      bool                             `json:"partial"`
	Count        int                              `json:"count"`
	IsManual     bool                             `json:"is_manual"`
	RequestedBy  *string                          `json:"requested_by,omitempty"`
	Matches      []ScreeningMatchDto              `json:"matches"`
	ErrorCodes   []string                         `json:"error_codes,omitempty"`
}

type ScreeningConfigRefDto struct {
	Name string `json:"name"`
}

type ScreeningRequestDto struct {
	Datasets    []string        `json:"datasets,omitempty"`
	Limit       int             `json:"limit,omitempty"`
	Threshold   int             `json:"threshold,omitempty"`
	SearchInput json.RawMessage `json:"search_input"`
}

func AdaptScreeningDto(m models.ScreeningWithMatches) ScreeningDto {
	screening := ScreeningDto{
		Id: m.Id,
		Config: ScreeningConfigRefDto{
			Name: m.Config.Name,
		},
		Status:      m.Status.String(),
		Partial:     m.Partial,
		Count:       m.NumberOfMatches,
		IsManual:    m.IsManual,
		RequestedBy: m.RequestedBy,
		Matches:     pure_utils.Map(m.Matches, AdaptScreeningMatchDto),
		ErrorCodes:  m.ErrorCodes,
	}
	if m.SearchInput != nil {
		screening.Request = &ScreeningRequestDto{
			Datasets:    m.Datasets,
			Limit:       m.OrgConfig.MatchLimit,
			Threshold:   m.OrgConfig.MatchThreshold,
			SearchInput: m.SearchInput,
		}
	}
	if m.Screening.InitialQuery != nil {
		screening.InitialQuery = m.InitialQuery
	}

	return screening
}

type ScreeningRefineDto struct {
	ScreeningId string         `json:"sanction_check_id"` //nolint:tagliatelle
	Query       RefineQueryDto `json:"query"`
}

func AdaptScreeningRefineDto(dto ScreeningRefineDto) models.ScreeningRefineRequest {
	return models.ScreeningRefineRequest{
		ScreeningId: dto.ScreeningId,
		Type:        dto.Query.Type(),
		Query:       AdaptRefineQueryDto(dto.Query),
	}
}

type ScreeningMatchDto struct {
	Id                           string                     `json:"id"`
	EntityId                     string                     `json:"entity_id"`
	Referents                    []string                   `json:"referents"`
	QueryIds                     []string                   `json:"query_ids"`
	Status                       string                     `json:"status"`
	ReviewedBy                   *string                    `json:"reviewer_id,omitempty"` //nolint:tagliatelle
	Datasets                     []string                   `json:"datasets"`
	UniqueCounterpartyIdentifier *string                    `json:"unique_counterparty_identifier"`
	Payload                      json.RawMessage            `json:"payload"`
	Enriched                     bool                       `json:"enriched"`
	Comments                     []ScreeningMatchCommentDto `json:"comments"`
}

func AdaptScreeningMatchDto(m models.ScreeningMatch) ScreeningMatchDto {
	match := ScreeningMatchDto{
		Id:                           m.Id,
		EntityId:                     m.EntityId,
		Referents:                    m.Referents,
		Status:                       m.Status.String(),
		ReviewedBy:                   m.ReviewedBy,
		QueryIds:                     m.QueryIds,
		Datasets:                     make([]string, 0),
		Payload:                      m.Payload,
		Enriched:                     m.Enriched,
		UniqueCounterpartyIdentifier: m.UniqueCounterpartyIdentifier,
		Comments:                     pure_utils.Map(m.Comments, AdaptScreeningMatchCommentDto),
	}

	return match
}

type ScreeningMatchUpdateDto struct {
	Status    string  `json:"status"`
	Comment   *string `json:"comment,omitempty"`
	Whitelist bool    `json:"whitelist"`
}

func AdaptScreeningMatchUpdateInputDto(matchId string, reviewerId models.UserId,
	dto ScreeningMatchUpdateDto,
) (models.ScreeningMatchUpdate, error) {
	if !slices.Contains(ValidScreeningMatchStatuses, dto.Status) {
		return models.ScreeningMatchUpdate{},
			errors.Wrap(models.BadParameterError, "invalid status for screening match")
	}

	update := models.ScreeningMatchUpdate{
		MatchId:    matchId,
		ReviewerId: &reviewerId,
		Status:     models.ScreeningMatchStatusFrom(dto.Status),
		Whitelist:  dto.Whitelist,
	}

	if dto.Comment != nil {
		update.Comment = &models.ScreeningMatchComment{
			MatchId:     matchId,
			CommenterId: reviewerId,
			Comment:     *dto.Comment,
		}
	}

	return update, nil
}

type ScreeningMatchCommentDto struct {
	Id        string    `json:"id"`
	AuthorId  string    `json:"author_id"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func AdaptScreeningMatchCommentInputDto(matchId string, commenterId models.UserId,
	m ScreeningMatchCommentDto,
) (models.ScreeningMatchComment, error) {
	match := models.ScreeningMatchComment{
		MatchId:     matchId,
		CommenterId: commenterId,
		Comment:     m.Comment,
	}

	return match, nil
}

func AdaptScreeningMatchCommentDto(m models.ScreeningMatchComment) ScreeningMatchCommentDto {
	match := ScreeningMatchCommentDto{
		Id:        m.Id,
		AuthorId:  string(m.CommenterId),
		Comment:   m.Comment,
		CreatedAt: m.CreatedAt,
	}

	return match
}

type ScreeningFileDto struct {
	Id        string    `json:"id"`
	Filename  string    `json:"filename"`
	CreatedAt time.Time `json:"created_at"`
}

func AdaptScreeningFileDto(m models.ScreeningFile) ScreeningFileDto {
	return ScreeningFileDto{
		Id:        m.Id,
		Filename:  m.FileName,
		CreatedAt: m.CreatedAt,
	}
}
