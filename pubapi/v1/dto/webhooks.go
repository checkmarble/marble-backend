package dto

import (
	"context"
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type WebhookEventPayload struct {
	Type      string           `json:"type"`
	Content   WebhookEventData `json:"content"`
	Timestamp time.Time        `json:"timestamp"`
}

func (p WebhookEventPayload) ApiVersion() string {
	if p.Content.Case != nil {
		return p.Content.Case.ApiVersion()
	}

	return "v1"
}

type WebhookEventData struct {
	Decision            *Decision                 `json:"decision,omitzero"`
	Case                *Case                     `json:"case,omitzero"`
	Files               *[]CaseFile               `json:"files,omitempty"`
	Comments            *CaseComment              `json:"comments,omitempty"`
	FailedAsyncDecision *FailedAsyncDecisionEvent `json:"failed_async_decision,omitzero"`
}

type FailedAsyncDecisionEvent struct {
	Id            uuid.UUID       `json:"id"`
	ObjectType    string          `json:"object_type"`
	ScenarioId    *string         `json:"scenario_id"`
	Stage         string          `json:"stage"`
	TriggerObject json.RawMessage `json:"trigger_object"`
	ErrorMessage  string          `json:"error_message"`
}

func AdaptWebhookEventData(ctx context.Context, exec repositories.Executor,
	adapter types.PublicApiDataAdapter, m models.WebhookEventPayload,
) (string, json.RawMessage, error) {
	users, err := adapter.ListUsers(ctx, exec)
	if err != nil {
		return "", nil, err
	}

	tags, err := adapter.ListTags(ctx, exec)
	if err != nil {
		return "", nil, err
	}

	refs := make(map[string]models.CaseReferents)

	if m.Content.Case != nil && m.Content.Case.Id != "" {
		re, err := adapter.GetCaseReferents(ctx, exec, []string{m.Content.Case.Id})
		if err != nil {
			return "", nil, err
		}
		for _, r := range re {
			refs[r.Id] = r
		}
	}

	payload := WebhookEventPayload{
		Type: string(m.Type),
		Content: WebhookEventData{
			Decision: applyWebhookEventData(m.Content.Decision, func(
				d models.DecisionWithRuleExecutions,
			) Decision {
				return AdaptDecision(true, m.Content.Decision.RuleExecutions,
					m.Content.Decision.ScreeningExecutions)(m.Content.Decision.Decision)
			}),
			Case: applyWebhookEventData(m.Content.Case, func(c models.Case) Case {
				return AdaptCase(users, tags, refs)(c)
			}),
			Files: applyWebhookEventData(m.Content.Files, func(f []models.CaseFile) []CaseFile {
				return pure_utils.Map(f, func(f models.CaseFile) CaseFile {
					return AdaptCaseFile(f)
				})
			}),
			Comments: applyWebhookEventData(m.Content.Comments, func(c models.CaseEvent) CaseComment {
				return AdaptCaseComment(users)(models.CaseCommentEvent{
					Id:        c.Id,
					UserId:    c.UserId,
					CreatedAt: c.CreatedAt,
					Comment:   c.AdditionalNote,
				})
			}),
			FailedAsyncDecision: applyWebhookEventData(m.Content.FailedAsyncDecision, func(
				d models.FailedAsyncDecisionEvent,
			) FailedAsyncDecisionEvent {
				return FailedAsyncDecisionEvent{
					Id:            d.AsyncDecisionExecutionId,
					ObjectType:    d.ObjectType,
					ScenarioId:    d.ScenarioId,
					Stage:         d.Stage.String(),
					TriggerObject: d.TriggerObject,
					ErrorMessage:  d.ErrorMessage,
				}
			}),
		},
		Timestamp: m.Timestamp,
	}

	out, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}

	return payload.ApiVersion(), out, nil
}

func applyWebhookEventData[I, O any](data *I, cb func(I) O) *O {
	if data == nil {
		return nil
	}
	return utils.Ptr(cb(*data))
}
