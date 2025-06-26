package agent_dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/guregu/null/v5"
)

type Case struct {
	CreatedAt    time.Time   `json:"created_at"`
	InboxName    string      `json:"inbox_name"`
	Name         string      `json:"name"`
	Status       string      `json:"status"`
	Outcome      string      `json:"outcome"`
	Tags         []string    `json:"tags"`
	SnoozedUntil *time.Time  `json:"remind_me_at,omitempty"` //nolint:tagliatelle
	AssignedTo   string      `json:"assigned_to,omitempty"`
	Boost        string      `json:"boost,omitempty"`
	Events       []CaseEvent `json:"events,omitempty"`
}

func AdaptCaseDto(c models.Case, tags []models.Tag, inboxes []models.Inbox, users []models.User) Case {
	inboxName := ""
	for _, inbox := range inboxes {
		if inbox.Id == c.InboxId {
			inboxName = inbox.Name
			break
		}
	}
	dto := Case{
		CreatedAt: c.CreatedAt,
		InboxName: inboxName,
		Name:      c.Name,
		Status:    c.Status.EnrichedStatus(c.SnoozedUntil, c.Boost),
		Outcome:   string(c.Outcome),
		Tags: pure_utils.Map(c.Tags, func(t models.CaseTag) string {
			for _, tag := range tags {
				if tag.Id == t.TagId {
					return tag.Name
				}
			}
			return ""
		}),
		Boost: c.Boost.String(),
		Events: pure_utils.Map(c.Events, func(e models.CaseEvent) CaseEvent {
			return AdaptCaseEventDto(e, users)
		}),
	}

	if c.SnoozedUntil != nil && c.SnoozedUntil.After(time.Now()) {
		dto.SnoozedUntil = c.SnoozedUntil
	}
	if c.AssignedTo != nil {
		for _, user := range users {
			if user.UserId == *c.AssignedTo {
				dto.AssignedTo = user.FullName()
				break
			}
		}
	}

	return dto
}

type CaseEvent struct {
	UserName       null.String `json:"user_name"`
	CreatedAt      time.Time   `json:"created_at"`
	EventType      string      `json:"event_type"`
	AdditionalNote string      `json:"additional_note"`
	NewValue       string      `json:"new_value"`
	ResourceType   string      `json:"resource_type"`
}

func AdaptCaseEventDto(caseEvent models.CaseEvent, users []models.User) CaseEvent {
	var userName null.String
	for _, user := range users {
		if user.UserId == models.UserId(caseEvent.UserId.String) {
			userName = null.StringFrom(user.FullName())
			break
		}
	}
	return CaseEvent{
		UserName:       userName,
		CreatedAt:      caseEvent.CreatedAt,
		EventType:      string(caseEvent.EventType),
		AdditionalNote: caseEvent.AdditionalNote,
		NewValue:       caseEvent.NewValue,
		ResourceType:   string(caseEvent.ResourceType),
	}
}

func AdaptCaseWithDecisionsDto(
	c models.Case,
	tags []models.Tag,
	inboxes []models.Inbox,
	rules []models.Rule,
	users []models.User,
	getScenarioIteration func(scenarioIterationId string) (models.ScenarioIteration, error),
	getScreenings func(decisionId string) ([]models.ScreeningWithMatches, error),
) (CaseWithDecisions, error) {
	decisions := make([]Decision, len(c.Decisions))
	for i := range c.Decisions {
		iteration, err := getScenarioIteration(c.Decisions[i].ScenarioIterationId)
		if err != nil {
			return CaseWithDecisions{}, err
		}
		screenings, err := getScreenings(c.Decisions[i].DecisionId)
		if err != nil {
			return CaseWithDecisions{}, err
		}
		decisions[i] = AdaptDecision(c.Decisions[i].Decision, iteration,
			c.Decisions[i].RuleExecutions, rules, screenings)
	}
	return CaseWithDecisions{
		Case:      AdaptCaseDto(c, tags, inboxes, users),
		Decisions: decisions,
	}, nil
}

type CaseWithDecisions struct {
	Case
	Decisions []Decision `json:"decisions"`
}

type IngestedDataResult struct {
	Data        []models.ClientObjectDetail `json:"data"`
	ReadOptions models.ExplorationOptions   `json:"-"`
}

type CasePivotObjectData struct {
	IngestedData map[string]IngestedDataResult `json:"ingested_data"`
	RelatedCases []CaseWithDecisions           `json:"related_cases"`
}
