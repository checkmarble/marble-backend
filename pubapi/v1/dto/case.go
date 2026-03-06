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
	return "v1beta"
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

type CaseReview struct {
	Id        string         `json:"id"`
	CaseId    string         `json:"case_id"`
	Status    string         `json:"status"`
	Reaction  *string        `json:"reaction"`
	CreatedAt types.DateTime `json:"created_at"`
	UpdatedAt types.DateTime `json:"updated_at"`
}

// CaseReviewContent is a marker interface for versioned case review content payloads.
// Implement this interface on each new version (CaseReviewContentV1, CaseReviewContentV2, …).
type CaseReviewContent interface {
	caseReviewContent()
}

type CaseReviewDetail struct {
	Id        string            `json:"id"`
	CaseId    string            `json:"case_id"`
	Status    string            `json:"status"`
	Reaction  *string           `json:"reaction"`
	CreatedAt types.DateTime    `json:"created_at"`
	UpdatedAt types.DateTime    `json:"updated_at"`
	Content   CaseReviewContent `json:"content"`
}

type CaseReviewContentV1 struct {
	Version string `json:"version"`
	Output  string `json:"output"`

	// SanityCheck is optional, it replace the `ok` field from the v1 response.
	// ok = true => `sanity_check = nil`
	// ok = false => `sanity_check = sanity check output`
	SanityCheck *string `json:"sanity_check"`
}

func (CaseReviewContentV1) caseReviewContent() {}

func AdaptCaseReview(r agent_dto.AiCaseReviewListItemDto) CaseReview {
	return CaseReview{
		Id:        r.Id.String(),
		CaseId:    r.CaseId.String(),
		Status:    r.Status,
		Reaction:  r.Reaction,
		CreatedAt: types.DateTime(r.CreatedAt),
		UpdatedAt: types.DateTime(r.UpdatedAt),
	}
}

func adaptCaseReviewContent(review agent_dto.AiCaseReviewDto) CaseReviewContent {
	switch v := review.(type) {
	case agent_dto.CaseReviewV1:
		sanityCheck := v.SanityCheck
		return CaseReviewContentV1{
			Version:     v.GetVersion(),
			Output:      v.Output,
			SanityCheck: &sanityCheck,
		}
		// future versions: case agent_dto.CaseReviewV2: return CaseReviewContentV2{...}
	}
	return nil
}

func AdaptCaseReviewDetail(r agent_dto.AiCaseReviewOutputDto) CaseReviewDetail {
	out := CaseReviewDetail{
		Id:        r.Id.String(),
		CaseId:    r.CaseId.String(),
		Status:    r.Status,
		Reaction:  r.Reaction,
		CreatedAt: types.DateTime(r.CreatedAt),
		UpdatedAt: types.DateTime(r.UpdatedAt),
	}
	if r.Review != nil {
		out.Content = adaptCaseReviewContent(r.Review)
	}
	return out
}
