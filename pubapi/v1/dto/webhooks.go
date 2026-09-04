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
	if p.Content.ContinuousScreening != nil {
		return p.Content.ContinuousScreening.ApiVersion()
	}
	if p.Content.Ingestion != nil {
		return "v1beta"
	}

	return "v1"
}

type WebhookEventData struct {
	Decision            *Decision                 `json:"decision,omitzero"`
	Case                *Case                     `json:"case,omitzero"`
	Files               *[]CaseFile               `json:"files,omitempty"`
	Comments            *CaseComment              `json:"comments,omitempty"`
	AsyncDecision       *AsyncDecisionExecution   `json:"async_decision,omitzero"`
	ContinuousScreening *ContinuousScreening      `json:"continuous_screening,omitzero"`
	Match               *ContinuousScreeningMatch `json:"match,omitzero"`
	RiskLevel           *RiskLevel                `json:"risk_level,omitzero"`
	Ingestion           *WebhookIngestion         `json:"ingestion,omitzero"`
}

type WebhookIngestion struct {
	Id           string     `json:"id"`
	ObjectType   string     `json:"object_type"`
	Status       string     `json:"status"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	RowsIngested int        `json:"rows_ingested"`
	ErrorCode    string     `json:"error_code,omitempty"`
	InputError   string     `json:"input_error,omitempty"`
}

func AdaptWebhookEventData(
	ctx context.Context,
	exec repositories.Executor,
	adapter types.PublicApiDataAdapter,
	m models.WebhookEventPayload,
) (string, json.RawMessage, error) {
	var users []models.User
	var tags []models.Tag
	if m.Content.Case != nil || m.Content.Comments != nil {
		var err error
		users, err = adapter.ListUsers(ctx, exec)
		if err != nil {
			return "", nil, err
		}
	}
	if m.Content.Case != nil {
		var err error
		tags, err = adapter.ListTags(ctx, exec)
		if err != nil {
			return "", nil, err
		}
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

	var matchTriggerType models.ContinuousScreeningTriggerType
	if m.Content.ContinuousScreening != nil {
		matchTriggerType = m.Content.ContinuousScreening.TriggerType
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
			AsyncDecision:       applyWebhookEventData(m.Content.AsyncDecisionExecution, AdaptAsyncDecisionExecution),
			ContinuousScreening: applyWebhookEventData(m.Content.ContinuousScreening, AdaptContinuousScreening),
			Match: applyWebhookEventData(m.Content.ContinuousScreeningMatch, func(match models.ContinuousScreeningMatch) ContinuousScreeningMatch {
				return AdaptContinuousScreeningMatch(matchTriggerType, match)
			}),
			RiskLevel: applyWebhookEventData(m.Content.Score, func(rl models.ScoringScore) RiskLevel {
				return AdaptRiskLevel(rl, nil)
			}),
			Ingestion: applyWebhookEventData(m.Content.Ingestion, func(upload models.UploadLog) WebhookIngestion {
				inputError := ""
				if upload.InputError != nil {
					inputError = *upload.InputError
				}
				errorCode := ""
				if upload.UploadStatus == models.UploadFailure {
					errorCode = "ingestion_failed"
				}
				return WebhookIngestion{
					Id: upload.Id.String(), ObjectType: upload.TableName, Status: string(upload.UploadStatus),
					StartedAt: upload.StartedAt, FinishedAt: upload.FinishedAt,
					RowsIngested: upload.RowsIngested, ErrorCode: errorCode, InputError: inputError,
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
