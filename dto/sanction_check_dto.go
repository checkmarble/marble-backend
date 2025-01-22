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
	Id          string                  `json:"id"`
	Datasets    []string                `json:"datasets"`
	Status      string                  `json:"status"`
	Request     SanctionCheckRequestDto `json:"request"`
	Partial     bool                    `json:"partial"`
	Count       int                     `json:"count"`
	IsManual    bool                    `json:"is_manual"`
	RequestedBy *string                 `json:"requested_by,omitempty"`
	Matches     []SanctionCheckMatchDto `json:"matches"`
}

type SanctionCheckRequestDto struct {
	Datasets  []string        `json:"datasets,omitempty"`
	Limit     *int            `json:"limit,omitempty"`
	Threshold *int            `json:"threshold,omitempty"`
	Query     json.RawMessage `json:"query"`
}

func AdaptSanctionCheckDto(m models.SanctionCheck) SanctionCheckDto {
	sanctionCheck := SanctionCheckDto{
		Id:       m.Id,
		Datasets: make([]string, 0),
		Request: SanctionCheckRequestDto{
			Datasets:  m.OrgConfig.Datasets,
			Limit:     m.OrgConfig.MatchLimit,
			Threshold: m.OrgConfig.MatchThreshold,
			Query:     m.Query,
		},
		Status:      m.Status,
		Partial:     m.Partial,
		Count:       m.Count,
		IsManual:    m.IsManual,
		RequestedBy: m.RequestedBy,
		Matches:     make([]SanctionCheckMatchDto, 0),
	}

	if len(m.OrgConfig.Datasets) > 0 {
		sanctionCheck.Datasets = m.OrgConfig.Datasets
	}
	if len(m.Matches) > 0 {
		sanctionCheck.Matches = pure_utils.Map(m.Matches, AdaptSanctionCheckMatchDto)
	}

	return sanctionCheck
}

type SanctionCheckMatchDto struct {
	Id           string          `json:"id"`
	EntityId     string          `json:"entity_id"`
	QueryIds     []string        `json:"query_ids"`
	Status       string          `json:"status"`
	ReviewedBy   *string         `json:"reviewer_id,omitempty"` //nolint:tagliatelle
	Datasets     []string        `json:"datasets"`
	Payload      json.RawMessage `json:"payload"`
	CommentCount int             `json:"comment_count"`
}

func AdaptSanctionCheckMatchDto(m models.SanctionCheckMatch) SanctionCheckMatchDto {
	match := SanctionCheckMatchDto{
		Id:           m.Id,
		EntityId:     m.EntityId,
		Status:       m.Status,
		ReviewedBy:   m.ReviewedBy,
		QueryIds:     m.QueryIds,
		Datasets:     make([]string, 0),
		Payload:      m.Payload,
		CommentCount: m.CommentCount,
	}

	return match
}

type SanctionCheckMatchUpdateDto struct {
	Status string `json:"status"`
}

func (dto SanctionCheckMatchUpdateDto) Validate() error {
	if !slices.Contains(ValidSanctionCheckMatchStatuses, dto.Status) {
		return errors.Wrap(models.BadParameterError,
			"invalid status for sanction check match")
	}

	return nil
}

type SanctionCheckMatchCommentDto struct {
	Id        string    `json:"id"`
	AuthorId  string    `json:"author_id"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
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
