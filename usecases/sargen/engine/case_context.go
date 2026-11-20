package engine

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// Bounds of the case context given to the tasks that ask for it.
const (
	CaseContextMaxDecisions = 50
	CaseContextMaxEvents    = 100
	CaseContextMaxChars     = 20000
	caseContextMaxNote      = 1000
)

// CaseContextReader is the subset of the case repositories the case context
// is built from. It is optional: without it no task receives a case context.
type CaseContextReader interface {
	DecisionsByCaseIdFromCursor(ctx context.Context, exec repositories.Executor,
		req models.CaseDecisionsRequest) ([]models.DecisionWithRulesAndScreeningsBaseInfo, bool, error)
	ListCaseEvents(ctx context.Context, exec repositories.Executor, caseId string) ([]models.CaseEvent, error)
}

type caseCtxRule struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	ScoreModifier int    `json:"score_modifier"`
}

type caseCtxScreening struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Matches int    `json:"matches"`
}

type caseCtxDecision struct {
	CreatedAt  time.Time          `json:"created_at"`
	Scenario   string             `json:"scenario"`
	Outcome    string             `json:"outcome"`
	Score      int                `json:"score"`
	Object     string             `json:"object,omitempty"`
	ObjectID   string             `json:"object_id,omitempty"`
	Rules      []caseCtxRule      `json:"rules,omitempty"`
	Screenings []caseCtxScreening `json:"screenings,omitempty"`
}

type caseCtxEvent struct {
	CreatedAt time.Time `json:"created_at"`
	Type      string    `json:"type"`
	Note      string    `json:"note,omitempty"`
	NewValue  string    `json:"new_value,omitempty"`
}

type caseCtx struct {
	Decisions []caseCtxDecision `json:"decisions"`
	Events    []caseCtxEvent    `json:"events"`
	Truncated bool              `json:"truncated,omitempty"`
}

// buildCaseContext reads the decisions (with their triggered rules and
// screenings) and the events of the case with the declaration's organization,
// newest first, and renders them as compact JSON of at most CaseContextMaxChars
// characters (the oldest events, then decisions, are dropped to fit).
func buildCaseContext(ctx context.Context, exec repositories.Executor, reader CaseContextReader,
	orgId, caseId uuid.UUID,
) (string, error) {
	decisions, hasMore, err := reader.DecisionsByCaseIdFromCursor(ctx, exec, models.CaseDecisionsRequest{
		OrgId: orgId, CaseId: caseId.String(), Limit: CaseContextMaxDecisions,
	})
	if err != nil {
		return "", errors.Wrap(err, "cannot read case decisions")
	}
	events, err := reader.ListCaseEvents(ctx, exec, caseId.String())
	if err != nil {
		return "", errors.Wrap(err, "cannot read case events")
	}
	out := caseCtx{Truncated: hasMore}
	for i, d := range decisions {
		if i >= CaseContextMaxDecisions {
			out.Truncated = true
			break
		}
		cd := caseCtxDecision{
			CreatedAt: d.CreatedAt, Scenario: d.ScenarioName, Outcome: d.Outcome.String(), Score: d.Score,
			Object: d.ClientObject.TableName, ObjectID: objectIdOf(d.ClientObject.Data),
		}
		for _, r := range d.RuleExecutions {
			if r.Outcome == "hit" {
				cd.Rules = append(cd.Rules, caseCtxRule{
					Name: r.Rule.Name, Description: shorten(r.Rule.Description, 300), ScoreModifier: r.ResultScoreModifier})
			}
		}
		for _, s := range d.ScreeningExecutions {
			cd.Screenings = append(cd.Screenings, caseCtxScreening{Name: s.Name, Status: s.Status.String(), Matches: s.NumberOfMatches})
		}
		out.Decisions = append(out.Decisions, cd)
	}
	for i, e := range events { // ListCaseEvents is newest first
		if i >= CaseContextMaxEvents {
			out.Truncated = true
			break
		}
		out.Events = append(out.Events, caseCtxEvent{
			CreatedAt: e.CreatedAt, Type: string(e.EventType),
			Note: shorten(e.AdditionalNote, caseContextMaxNote), NewValue: shorten(e.NewValue, 200),
		})
	}
	if out.Decisions == nil {
		out.Decisions = []caseCtxDecision{}
	}
	if out.Events == nil {
		out.Events = []caseCtxEvent{}
	}
	for {
		raw, err := json.Marshal(out)
		if err != nil {
			return "", errors.Wrap(err, "cannot encode case context")
		}
		if len([]rune(string(raw))) <= CaseContextMaxChars {
			return string(raw), nil
		}
		out.Truncated = true
		switch {
		case len(out.Events) > 0:
			out.Events = out.Events[:len(out.Events)-1]
		case len(out.Decisions) > 0:
			out.Decisions = out.Decisions[:len(out.Decisions)-1]
		default:
			return `{"decisions":[],"events":[],"truncated":true}`, nil
		}
	}
}

func objectIdOf(data map[string]any) string {
	if id, ok := valueString(data["object_id"]); ok {
		return id
	}
	return ""
}
