package dto

import (
	"fmt"

	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type Case struct {
	Id           string          `json:"id"`
	Inbox        Ref             `json:"inbox"`
	Name         string          `json:"name"`
	Assignee     *Ref            `json:"assignee"`
	Status       string          `json:"status"`
	Outcome      string          `json:"outcome"`
	Contributors []Ref           `json:"contributors"`
	Tags         []Ref           `json:"tags"`
	SnoozedUntil *types.DateTime `json:"snoozed_until,omitempty"`
	ReviewLevel  *string         `json:"review_level"`
	CreatedAt    types.DateTime  `json:"created_at"`
}

func (Case) ApiVersion() string {
	return "v1"
}

type CaseComment struct {
	Id        string         `json:"id"`
	User      Ref            `json:"user"`
	Comment   string         `json:"comment"`
	CreatedAt types.DateTime `json:"created_at"`
}

type CaseFile struct {
	Id        string         `json:"id"`
	Filename  string         `json:"file_name"` //nolint:tagliatelle
	CreatedAt types.DateTime `json:"created_at"`
}

func AdaptCase(users []models.User, tags []models.Tag, referents map[string]models.CaseReferents) func(c models.Case) Case {
	userMap := pure_utils.MapSliceToMap(users, func(u models.User) (models.UserId, models.User) { return u.UserId, u })
	tagMap := pure_utils.MapSliceToMap(tags, func(t models.Tag) (string, models.Tag) { return t.Id, t })

	return func(c models.Case) Case {
		out := Case{
			Id:           c.Id,
			Name:         c.Name,
			Status:       string(c.Status),
			Outcome:      string(c.Outcome),
			SnoozedUntil: types.ThenDateTime(c.SnoozedUntil),
			CreatedAt:    types.DateTime(c.CreatedAt),
			Contributors: make([]Ref, 0),
			Tags:         make([]Ref, 0),
			ReviewLevel:  c.ReviewLevel,
		}

		if ref, ok := referents[c.Id]; ok {
			out.Inbox = AdaptInboxRef(ref.Inbox)

			if ref.Assignee != nil {
				out.Assignee = utils.Ptr(AdaptUserRef(*ref.Assignee))
			}
		}

		for _, contrib := range c.Contributors {
			if u, ok := userMap[models.UserId(contrib.UserId)]; ok {
				out.Contributors = append(out.Contributors, AdaptUserRef(u))
			}
		}
		for _, tag := range c.Tags {
			if t, ok := tagMap[tag.TagId]; ok {
				out.Tags = append(out.Tags, AdaptTagRef(t))
			}
		}

		return out
	}
}

func AdaptCaseComment(users []models.User) func(models.CaseCommentEvent) CaseComment {
	userMap := pure_utils.MapSliceToMap(users, func(u models.User) (models.UserId, models.User) { return u.UserId, u })

	return func(c models.CaseCommentEvent) CaseComment {
		comment := CaseComment{
			Id:        c.Id,
			Comment:   c.Comment,
			CreatedAt: types.DateTime(c.CreatedAt),
			User: Ref{
				Id:   uuid.Nil.String(),
				Name: "unknown user",
			},
		}

		if c.UserId.IsZero() {
			comment.User.Name = "system"
		}

		if author, ok := userMap[models.UserId(c.UserId.ValueOrZero())]; ok {
			comment.User = Ref{
				Id:   string(author.UserId),
				Name: fmt.Sprintf("%s %s", author.FirstName, author.LastName),
			}
		}

		return comment
	}
}

func AdaptCaseFile(f models.CaseFile) CaseFile {
	file := CaseFile{
		Id:        f.Id,
		Filename:  f.FileName,
		CreatedAt: types.DateTime(f.CreatedAt),
	}

	return file
}

type CaseAiReview struct {
	Id        string         `json:"id"`
	CaseId    string         `json:"case_id"`
	Status    string         `json:"status"`
	Reaction  *string        `json:"reaction"`
	CreatedAt types.DateTime `json:"created_at"`
	UpdatedAt types.DateTime `json:"updated_at"`
}

// CaseAiReviewContent is a marker interface for versioned AI case review content payloads.
// Implement this interface on each new version (CaseAiReviewContentV1, CaseAiReviewContentV2, …).
type CaseAiReviewContent interface {
	caseAiReviewContent()
}

type CaseAiReviewDetail struct {
	Id        string              `json:"id"`
	CaseId    string              `json:"case_id"`
	Status    string              `json:"status"`
	Reaction  *string             `json:"reaction"`
	CreatedAt types.DateTime      `json:"created_at"`
	UpdatedAt types.DateTime      `json:"updated_at"`
	Content   CaseAiReviewContent `json:"content"`
}

type CaseAiReviewContentV1 struct {
	Version string `json:"version"`
	Output  string `json:"output"`

	// SanityCheck is optional, it replace the `ok` field from the v1 response.
	// ok = true => `sanity_check = nil`
	// ok = false => `sanity_check = sanity check output`
	SanityCheck *string `json:"sanity_check"`
}

func (CaseAiReviewContentV1) caseAiReviewContent() {}

func AdaptAiCaseReview(r agent_dto.AiCaseReviewListItemDto) CaseAiReview {
	return CaseAiReview{
		Id:        r.Id.String(),
		CaseId:    r.CaseId.String(),
		Status:    r.Status,
		Reaction:  r.Reaction,
		CreatedAt: types.DateTime(r.CreatedAt),
		UpdatedAt: types.DateTime(r.UpdatedAt),
	}
}

func adaptAiCaseReviewContent(review agent_dto.AiCaseReviewDto) CaseAiReviewContent {
	switch v := review.(type) {
	case agent_dto.CaseReviewV1:
		var sanityCheck *string
		if v.SanityCheck != "" {
			sanityCheck = &v.SanityCheck
		}
		return CaseAiReviewContentV1{
			Version:     v.GetVersion(),
			Output:      v.Output,
			SanityCheck: sanityCheck,
		}
		// future versions: case agent_dto.CaseReviewV2: return CaseReviewContentV2{...}
	}
	return nil
}

func AdaptAiCaseReviewDetail(r agent_dto.AiCaseReviewOutputDto) CaseAiReviewDetail {
	out := CaseAiReviewDetail{
		Id:        r.Id.String(),
		CaseId:    r.CaseId.String(),
		Status:    r.Status,
		Reaction:  r.Reaction,
		CreatedAt: types.DateTime(r.CreatedAt),
		UpdatedAt: types.DateTime(r.UpdatedAt),
	}
	if r.Review != nil {
		out.Content = adaptAiCaseReviewContent(r.Review)
	}
	return out
}
