package agent_dto

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/guregu/null/v5"
)

type Case struct {
	Id           string      `json:"id"`
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
		Id:        c.Id,
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

func AdaptCaseWithDecisionsDtoWithoutRuleExecDetails(
	c models.Case,
	decisions []models.DecisionWithRulesAndScreeningsBaseInfo,
	tags []models.Tag,
	inboxes []models.Inbox,
	users []models.User,
	getScenarioIteration func(scenarioIterationId string) (models.ScenarioIteration, error),
) (CaseWithDecisions, error) {
	decisionDtos := make([]Decision, len(c.Decisions))
	for i := range c.Decisions {
		iteration, err := getScenarioIteration(c.Decisions[i].ScenarioIterationId.String())
		if err != nil {
			return CaseWithDecisions{}, err
		}
		decisionDtos[i] = AdaptDecisionWithoutRuleExecDetails(decisions[i], iteration)
	}
	return CaseWithDecisions{
		Case:      AdaptCaseDto(c, tags, inboxes, users),
		Decisions: decisionDtos,
	}, nil
}

type CaseWithDecisions struct {
	Case
	Decisions []Decision `json:"decisions"`
}

type IngestedDataResult struct {
	Data        []models.ClientObjectDetail
	ReadOptions models.ExplorationOptions
}

func (i IngestedDataResult) PrintForAgent() (string, error) {
	stringBuilder := strings.Builder{}
	csvWriter := csv.NewWriter(&stringBuilder)

	err := WriteClientDataToCsv(i.Data, csvWriter)
	if err != nil {
		return "", errors.Wrap(err, "could not write client data to csv")
	}
	csvWriter.Flush()
	return stringBuilder.String(), nil
}

type CasePivotIngestedData map[string]IngestedDataResult

func (c CasePivotIngestedData) PrintForAgent() (string, error) {
	stringBuilder := strings.Builder{}

	for key, value := range c {
		ingestedDataFormatted, err := value.PrintForAgent()
		if err != nil {
			return "", err
		}
		stringBuilder.WriteString(fmt.Sprintf("\nData from table \"%s\" as csv:\n <IngestedDataTable>\n%s\n</IngestedDataTable>", key, ingestedDataFormatted))
	}

	return stringBuilder.String(), nil
}

type CasIngestedDataByPivot map[string]CasePivotIngestedData

func (c CasIngestedDataByPivot) PrintForAgent() (string, error) {
	stringBuilder := strings.Builder{}

	for pivotValueKey, value := range c {
		pivotValue, pivotObjectType := pivotObjectTypeAndValueFromKey(pivotValueKey)
		allTablesFormatted, err := value.PrintForAgent()
		if err != nil {
			return "", err
		}
		stringBuilder.WriteString(fmt.Sprintf("\nAll ingested data for customer of type \"%s\" with pivot value \"%s\":\n <IngestedDataForPivot>\n%s\n</IngestedDataForPivot>",
			pivotObjectType, pivotValue, allTablesFormatted))
	}

	return stringBuilder.String(), nil
}

func PivotObjectKeyForMap(pivotObject models.PivotObject) string {
	return pivotObject.PivotObjectName + ":::" + pivotObject.PivotValue
}

func pivotObjectTypeAndValueFromKey(key string) (string, string) {
	parts := strings.Split(key, ":::")
	return parts[0], parts[1]
}
