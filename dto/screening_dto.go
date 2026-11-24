package dto

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
)

var (
	ValidScreeningStatuses      = []string{"in_review", "confirmed_hit", "error"}
	ValidScreeningMatchStatuses = []string{"pending", "confirmed_hit", "no_hit"}
)

type ScreeningAvailableFilters struct {
	Provider models.ScreeningProvider          `json:"provider"`
	Sections ScreeningAvailableFiltersSections `json:"sections"`
}

type ScreeningAvailableFiltersSections struct {
	Global       ScreeningAvailableFiltersSection `json:"global,omitzero"`
	Sanctions    ScreeningAvailableFiltersSection `json:"sanctions,omitzero"`
	Peps         ScreeningAvailableFiltersSection `json:"peps,omitzero"`
	AdverseMedia ScreeningAvailableFiltersSection `json:"adverse_media,omitzero"`
	Other        ScreeningAvailableFiltersSection `json:"other,omitzero"`
	Custom       ScreeningAvailableFiltersSection `json:"custom,omitzero"`
}

type ScreeningAvailableFiltersSection struct {
	Self     string                                     `json:"self,omitempty"`
	Datasets []ScreeningAvailableFiltersItem            `json:"datasets"`
	Topics   map[string][]ScreeningAvailableFiltersItem `json:"topics,omitempty"`
}

type ScreeningAvailableFiltersItem struct {
	Section string `json:"section,omitempty"`
	Name    string `json:"name"`
	Title   string `json:"title"`
}

type ScreeningDto struct {
	Id                           string                           `json:"id"`
	Config                       ScreeningConfigRefDto            `json:"config"`
	Status                       string                           `json:"status"`
	UniqueCounterpartyIdentifier *string                          `json:"unique_counterparty_identifier,omitempty"`
	Request                      *ScreeningRequestDto             `json:"request"`
	InitialQuery                 []models.OpenSanctionsCheckQuery `json:"initial_query"`
	Partial                      bool                             `json:"partial"`
	Count                        int                              `json:"count"`
	IsManual                     bool                             `json:"is_manual"`
	RequestedBy                  *string                          `json:"requested_by,omitempty"`
	Matches                      []ScreeningMatchDto              `json:"matches"`
	ErrorCodes                   []string                         `json:"error_codes,omitempty"`
}

type ScreeningConfigRefDto struct {
	Name     string                   `json:"name"`
	Provider models.ScreeningProvider `json:"provider"`
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
			Name:     m.Config.Name,
			Provider: m.Config.Provider,
		},
		Status:                       m.Status.String(),
		UniqueCounterpartyIdentifier: m.UniqueCounterpartyIdentifier,
		Partial:                      m.Partial,
		Count:                        m.NumberOfMatches,
		IsManual:                     m.IsManual,
		RequestedBy:                  m.RequestedBy,
		Matches:                      pure_utils.Map(m.Matches, AdaptScreeningMatchDto),
		ErrorCodes:                   m.ErrorCodes,
	}
	if m.SearchInput != nil {
		screening.Request = &ScreeningRequestDto{
			Datasets:    m.Config.Datasets,
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

type ScreeningFreeformDto struct {
	ScreeningRefineDto

	Datasets  []string                      `json:"datasets"`
	Filters   models.ScreeningConfigFilters `json:"filters"`
	Threshold *int                          `json:"threshold"`
}

type ScreeningRefineDto struct {
	ScreeningId string         `json:"screening_id"`
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
	Id         string                     `json:"id"`
	EntityId   string                     `json:"entity_id"`
	Referents  []string                   `json:"referents"`
	QueryIds   []string                   `json:"query_ids"`
	Status     string                     `json:"status"`
	ReviewedBy *string                    `json:"reviewer_id,omitempty"` //nolint:tagliatelle
	Datasets   []string                   `json:"datasets"`
	Payload    json.RawMessage            `json:"payload"`
	Enriched   bool                       `json:"enriched"`
	Comments   []ScreeningMatchCommentDto `json:"comments"`

	TentativeReview *ScreeningMatchTentativeReviewCommentDto `json:"tentative_review"`
}

func AdaptScreeningMatchDto(m models.ScreeningMatch) ScreeningMatchDto {
	match := ScreeningMatchDto{
		Id:         m.Id,
		EntityId:   m.EntityId,
		Referents:  m.Referents,
		Status:     m.Status.String(),
		ReviewedBy: m.ReviewedBy,
		QueryIds:   m.QueryIds,
		Datasets:   make([]string, 0),
		Payload:    m.Payload,
		Enriched:   m.Enriched,
	}

	match.Comments, match.TentativeReview = adaptScreeningMatchComments(m.Comments)

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

	if dto.Comment != nil && strings.TrimSpace(*dto.Comment) != "" {
		update.Comment = &models.ScreeningMatchComment{
			MatchId:     matchId,
			CommenterId: reviewerId,
			Comment:     *dto.Comment,
		}
	}

	return update, nil
}

type ScreeningMatchTentativeReviewDto struct {
	Status       string     `json:"status"`
	InboxId      string     `json:"inbox_id,omitempty"`
	SupersededAt *time.Time `json:"superseded_at,omitempty"`
}

type ScreeningMatchTentativeReviewCommentDto struct {
	Id        string    `json:"id"`
	AuthorId  string    `json:"author_id"`
	Comment   string    `json:"comment"`
	Status    string    `json:"status"`
	InboxId   string    `json:"inbox_id"`
	CreatedAt time.Time `json:"created_at"`
}

func adaptScreeningMatchComments(comments []models.ScreeningMatchComment) ([]ScreeningMatchCommentDto, *ScreeningMatchTentativeReviewCommentDto) {
	regular := make([]ScreeningMatchCommentDto, 0, len(comments))

	var review *ScreeningMatchTentativeReviewCommentDto

	for _, c := range comments {
		if c.TentativeReview == nil || c.TentativeReview.SupersededAt != nil {
			regular = append(regular, AdaptScreeningMatchCommentDto(c))

			continue
		}

		review = &ScreeningMatchTentativeReviewCommentDto{
			Id:        c.Id,
			AuthorId:  string(c.CommenterId),
			Comment:   c.Comment,
			Status:    c.TentativeReview.Status,
			InboxId:   c.TentativeReview.InboxId.String(),
			CreatedAt: c.CreatedAt,
		}
	}

	return regular, review
}

type ScreeningMatchCommentDto struct {
	Id              string                            `json:"id"`
	AuthorId        string                            `json:"author_id"`
	Comment         string                            `json:"comment"`
	TentativeReview *ScreeningMatchTentativeReviewDto `json:"tentative_review,omitempty"`
	CreatedAt       time.Time                         `json:"created_at"`
}

func AdaptScreeningMatchCommentInputDto(matchId string, m ScreeningMatchCommentDto) (models.ScreeningMatchComment, error) {
	if strings.TrimSpace(m.Comment) == "" {
		return models.ScreeningMatchComment{}, errors.Wrap(models.BadParameterError, "a comment cannot be empty")
	}

	match := models.ScreeningMatchComment{
		MatchId: matchId,
		Comment: m.Comment,
	}

	if m.TentativeReview != nil {
		match.TentativeReview = &models.ScreeningMatchTentativeReview{Status: m.TentativeReview.Status}
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

	if m.TentativeReview != nil {
		match.TentativeReview = &ScreeningMatchTentativeReviewDto{
			Status:       m.TentativeReview.Status,
			InboxId:      m.TentativeReview.InboxId.String(),
			SupersededAt: m.TentativeReview.SupersededAt,
		}
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
