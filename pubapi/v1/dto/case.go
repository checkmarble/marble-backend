package dto

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type Case struct {
	Id           string           `json:"id"`
	Inbox        Ref              `json:"inbox"`
	Name         string           `json:"name"`
	Assignee     *Ref             `json:"assignee"`
	Status       string           `json:"status"`
	Outcome      string           `json:"outcome"`
	Contributors []Ref            `json:"contributors"`
	Tags         []Ref            `json:"tags"`
	SnoozedUntil *pubapi.DateTime `json:"snoozed_until,omitempty"`
	CreatedAt    pubapi.DateTime  `json:"created_at"`
}

type CaseComment struct {
	Id        string          `json:"id"`
	User      Ref             `json:"user"`
	Comment   string          `json:"comment"`
	CreatedAt pubapi.DateTime `json:"created_at"`
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
			SnoozedUntil: pubapi.ThenDateTime(c.SnoozedUntil),
			CreatedAt:    pubapi.DateTime(c.CreatedAt),
			Contributors: make([]Ref, 0),
			Tags:         make([]Ref, 0),
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

func AdaptCaseComment(users []models.User) func(models.CaseEvent) CaseComment {
	userMap := pure_utils.MapSliceToMap(users, func(u models.User) (models.UserId, models.User) { return u.UserId, u })

	return func(c models.CaseEvent) CaseComment {
		comment := CaseComment{
			Id:        c.Id,
			Comment:   c.AdditionalNote,
			CreatedAt: pubapi.DateTime(c.CreatedAt),
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
