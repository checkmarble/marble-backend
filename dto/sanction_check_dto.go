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
	ValidSanctionCheckStatuses      = []string{"in_review", "confirmed_hit", "error"}
	ValidSanctionCheckMatchStatuses = []string{"pending", "confirmed_hit", "no_hit"}
)

type SanctionCheckDto struct {
	Id          string                    `json:"id"`
	Config      SanctionCheckConfigRefDto `json:"config"`
	Status      string                    `json:"status"`
	Request     *SanctionCheckRequestDto  `json:"request"`
	Partial     bool                      `json:"partial"`
	Count       int                       `json:"count"`
	IsManual    bool                      `json:"is_manual"`
	RequestedBy *string                   `json:"requested_by,omitempty"`
	Matches     []SanctionCheckMatchDto   `json:"matches"`
	ErrorCodes  []string                  `json:"error_codes,omitempty"`
}

type SanctionCheckConfigRefDto struct {
	Name string `json:"name"`
}

type SanctionCheckRequestDto struct {
	Datasets    []string        `json:"datasets,omitempty"`
	Limit       int             `json:"limit,omitempty"`
	Threshold   int             `json:"threshold,omitempty"`
	SearchInput json.RawMessage `json:"search_input"`
}

func AdaptSanctionCheckDto(m models.SanctionCheckWithMatches) SanctionCheckDto {
	sanctionCheck := SanctionCheckDto{
		Id: m.Id,
		Config: SanctionCheckConfigRefDto{
			Name: m.Config.Name,
		},
		Status:      m.Status.String(),
		Partial:     m.Partial,
		Count:       m.Count,
		IsManual:    m.IsManual,
		RequestedBy: m.RequestedBy,
		Matches:     pure_utils.Map(m.Matches, AdaptSanctionCheckMatchDto),
		ErrorCodes:  m.ErrorCodes,
	}
	if m.SearchInput != nil {
		sanctionCheck.Request = &SanctionCheckRequestDto{
			Datasets:    m.Datasets,
			Limit:       m.OrgConfig.MatchLimit,
			Threshold:   m.OrgConfig.MatchThreshold,
			SearchInput: m.SearchInput,
		}
	}

	return sanctionCheck
}

type SanctionCheckRefineDto struct {
	DecisionId string         `json:"decision_id"`
	Query      RefineQueryDto `json:"query"`
}

func AdaptSanctionCheckRefineDto(dto SanctionCheckRefineDto) models.SanctionCheckRefineRequest {
	return models.SanctionCheckRefineRequest{
		DecisionId: dto.DecisionId,
		Type:       dto.Query.Type(),
		Query:      AdaptRefineQueryDto(dto.Query),
	}
}

type SanctionCheckMatchDto struct {
	Id                           string                         `json:"id"`
	EntityId                     string                         `json:"entity_id"`
	QueryIds                     []string                       `json:"query_ids"`
	Status                       string                         `json:"status"`
	ReviewedBy                   *string                        `json:"reviewer_id,omitempty"` //nolint:tagliatelle
	Datasets                     []string                       `json:"datasets"`
	UniqueCounterpartyIdentifier *string                        `json:"unique_counterparty_identifier"`
	Payload                      json.RawMessage                `json:"payload"`
	Enriched                     bool                           `json:"enriched"`
	Comments                     []SanctionCheckMatchCommentDto `json:"comments"`
}

func AdaptSanctionCheckMatchDto(m models.SanctionCheckMatch) SanctionCheckMatchDto {
	match := SanctionCheckMatchDto{
		Id:                           m.Id,
		EntityId:                     m.EntityId,
		Status:                       m.Status.String(),
		ReviewedBy:                   m.ReviewedBy,
		QueryIds:                     m.QueryIds,
		Datasets:                     make([]string, 0),
		Payload:                      m.Payload,
		Enriched:                     m.Enriched,
		UniqueCounterpartyIdentifier: m.UniqueCounterpartyIdentifier,
		Comments:                     pure_utils.Map(m.Comments, AdaptSanctionCheckMatchCommentDto),
	}

	return match
}

type SanctionCheckMatchUpdateDto struct {
	Status    string  `json:"status"`
	Comment   *string `json:"comment,omitempty"`
	Whitelist bool    `json:"whitelist"`
}

func AdaptSanctionCheckMatchUpdateInputDto(matchId string, reviewerId models.UserId,
	dto SanctionCheckMatchUpdateDto,
) (models.SanctionCheckMatchUpdate, error) {
	if !slices.Contains(ValidSanctionCheckMatchStatuses, dto.Status) {
		return models.SanctionCheckMatchUpdate{},
			errors.Wrap(models.BadParameterError, "invalid status for sanction check match")
	}

	update := models.SanctionCheckMatchUpdate{
		MatchId:    matchId,
		ReviewerId: &reviewerId,
		Status:     models.SanctionCheckMatchStatusFrom(dto.Status),
		Whitelist:  dto.Whitelist,
	}

	if dto.Comment != nil {
		update.Comment = &models.SanctionCheckMatchComment{
			MatchId:     matchId,
			CommenterId: reviewerId,
			Comment:     *dto.Comment,
		}
	}

	return update, nil
}

type SanctionCheckMatchCommentDto struct {
	Id        string    `json:"id"`
	AuthorId  string    `json:"author_id"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func AdaptSanctionCheckMatchCommentInputDto(matchId string, commenterId models.UserId,
	m SanctionCheckMatchCommentDto,
) (models.SanctionCheckMatchComment, error) {
	match := models.SanctionCheckMatchComment{
		MatchId:     matchId,
		CommenterId: commenterId,
		Comment:     m.Comment,
	}

	return match, nil
}

func AdaptSanctionCheckMatchCommentDto(m models.SanctionCheckMatchComment) SanctionCheckMatchCommentDto {
	match := SanctionCheckMatchCommentDto{
		Id:        m.Id,
		AuthorId:  string(m.CommenterId),
		Comment:   m.Comment,
		CreatedAt: m.CreatedAt,
	}

	return match
}

type SanctionCheckFileDto struct {
	Id        string    `json:"id"`
	Filename  string    `json:"filename"`
	CreatedAt time.Time `json:"created_at"`
}

func AdaptSanctionCheckFileDto(m models.SanctionCheckFile) SanctionCheckFileDto {
	return SanctionCheckFileDto{
		Id:        m.Id,
		Filename:  m.FileName,
		CreatedAt: m.CreatedAt,
	}
}
