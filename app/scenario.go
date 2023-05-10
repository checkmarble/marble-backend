package app

import (
	"context"
	"errors"
	"marble/marble-backend/app/operators"
	"marble/marble-backend/utils"
	"runtime/debug"
	"time"

	"golang.org/x/exp/slog"
)

///////////////////////////////
// Scenario
///////////////////////////////

type Scenario struct {
	ID                string
	Name              string
	Description       string
	TriggerObjectType string
	CreatedAt         time.Time
	LiveVersionID     *string
}

type CreateScenarioInput struct {
	Name              string
	Description       string
	TriggerObjectType string
}

type UpdateScenarioInput struct {
	ID          string
	Name        *string
	Description *string
}

type PublishedScenarioIteration struct {
	ID         string
	ScenarioID string
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Body       PublishedScenarioIterationBody
}

type PublishedScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	Rules                []Rule
	ScoreReviewThreshold int
	ScoreRejectThreshold int
}

func NewPublishedScenarioIteration(si ScenarioIteration) (PublishedScenarioIteration, error) {
	result := PublishedScenarioIteration{
		ID:         si.ID,
		ScenarioID: si.ScenarioID,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
	}

	if si.Version == nil {
		return PublishedScenarioIteration{}, ErrScenarioIterationNotValid
	}
	result.Version = *si.Version

	if si.Body.ScoreReviewThreshold == nil {
		return PublishedScenarioIteration{}, ErrScenarioIterationNotValid
	}
	result.Body.ScoreReviewThreshold = *si.Body.ScoreReviewThreshold

	if si.Body.ScoreRejectThreshold == nil {
		return PublishedScenarioIteration{}, ErrScenarioIterationNotValid
	}
	result.Body.ScoreRejectThreshold = *si.Body.ScoreRejectThreshold

	if len(si.Body.Rules) < 1 {
		return PublishedScenarioIteration{}, ErrScenarioIterationNotValid
	}
	result.Body.Rules = si.Body.Rules

	if si.Body.TriggerCondition == nil {
		return PublishedScenarioIteration{}, ErrScenarioIterationNotValid
	}
	result.Body.TriggerCondition = si.Body.TriggerCondition

	return result, nil
}

func (si ScenarioIteration) IsValideForPublication() bool {
	if si.Body.ScoreReviewThreshold == nil {
		return false
	}

	if si.Body.ScoreRejectThreshold == nil {
		return false
	}

	if len(si.Body.Rules) < 1 {
		return false
	}

	if si.Body.TriggerCondition == nil {
		return false
	}

	return true
}

type ScenarioIteration struct {
	ID         string
	ScenarioID string
	Version    *int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Body       ScenarioIterationBody
}

type GetScenarioIterationFilters struct {
	ScenarioID *string
}

type ScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	Rules                []Rule
	ScoreReviewThreshold *int
	ScoreRejectThreshold *int
}

type CreateScenarioIterationInput struct {
	ScenarioID string
	Body       *CreateScenarioIterationBody
}

type CreateScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	Rules                []CreateRuleInput
	ScoreReviewThreshold *int
	ScoreRejectThreshold *int
}

type UpdateScenarioIterationInput struct {
	ID   string
	Body *UpdateScenarioIterationBody
}

type UpdateScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	ScoreReviewThreshold *int
	ScoreRejectThreshold *int
}

///////////////////////////////
// ScenarioExecution
///////////////////////////////

type ScenarioExecution struct {
	ScenarioID          string
	ScenarioName        string
	ScenarioDescription string
	ScenarioVersion     int
	RuleExecutions      []RuleExecution
	Score               int
	Outcome             Outcome
}

var (
	ErrPanicInScenarioEvalution                         = errors.New("panic during scenario evaluation")
	ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch   = errors.New("scenario's trigger_type and provided trigger_object type are different")
	ErrScenarioTriggerConditionAndTriggerObjectMismatch = errors.New("trigger_object does not match the scenario's trigger conditions")
	ErrScenarioHasNoLiveVersion                         = errors.New("scenario has no live version")
)

func (s Scenario) Eval(ctx context.Context, repo RepositoryInterface, payloadStructWithReader DynamicStructWithReader, dataModel DataModel, logger *slog.Logger) (se ScenarioExecution, err error) {

	///////////////////////////////
	// Recover in case the evaluation panicked.
	// Even if there is a "recoverer" middleware in our stack, this allows a sentinel error to be used and to catch the failure early
	///////////////////////////////
	defer func() {
		if r := recover(); r != nil {
			logger.WarnCtx(ctx, "recovered from panic during Eval. stacktrace from panic: ")
			logger.WarnCtx(ctx, string(debug.Stack()))

			err = ErrPanicInScenarioEvalution
			se = ScenarioExecution{}
		}
	}()

	logger.InfoCtx(ctx, "Evaluting scenario", "scenarioId", s.ID)

	// If the scenario has no live version, don't try to Eval() it, return early
	if s.LiveVersionID == nil {
		return ScenarioExecution{}, ErrScenarioHasNoLiveVersion
	}

	orgID, err := utils.OrgIDFromCtx(ctx)
	if err != nil {
		return ScenarioExecution{}, utils.ErrOrgNotInContext
	}
	liveVersion, err := repo.GetScenarioIteration(ctx, orgID, *s.LiveVersionID)

	// Check the scenario & trigger_object's types
	if s.TriggerObjectType != payloadStructWithReader.Table.Name {
		return ScenarioExecution{}, ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch
	}

	dataAccessor := DataAccessorImpl{DataModel: dataModel, Payload: payloadStructWithReader, repository: repo}

	// Evaluate the trigger
	triggerPassed, err := liveVersion.Body.TriggerCondition.Eval(&dataAccessor)
	if err != nil {
		return ScenarioExecution{}, err
	}

	if !triggerPassed {
		return ScenarioExecution{}, ErrScenarioTriggerConditionAndTriggerObjectMismatch
	}

	// Evaluate all rules
	score := 0
	ruleExecutions := make([]RuleExecution, 0)
	for _, rule := range liveVersion.Body.Rules {

		// Evaluate single rule
		ruleExecution, err := rule.Eval(&dataAccessor)
		if err != nil {
			return ScenarioExecution{}, err
		}
		logger.InfoCtx(ctx, "Rule executed", slog.Int("score_modifier", ruleExecution.Rule.ScoreModifier), slog.String("ruleName", ruleExecution.Rule.Formula.String()), slog.Bool("result", ruleExecution.Result))

		// Increment scenario score when rule is true
		if ruleExecution.Result {
			score += ruleExecution.Rule.ScoreModifier
		}

		ruleExecutions = append(ruleExecutions, ruleExecution)
	}

	// Compute outcome from score
	o := None

	if score < *liveVersion.Body.ScoreReviewThreshold {
		o = Approve
	}
	if score >= *liveVersion.Body.ScoreReviewThreshold && score < *liveVersion.Body.ScoreRejectThreshold {
		o = Review
	}
	if score > *liveVersion.Body.ScoreRejectThreshold {
		o = Reject
	}

	// Build ScenarioExecution as result
	se = ScenarioExecution{
		ScenarioID:          s.ID,
		ScenarioName:        s.Name,
		ScenarioDescription: s.Description,
		ScenarioVersion:     *liveVersion.Version,
		RuleExecutions:      ruleExecutions,
		Score:               score,
		Outcome:             o,
	}

	logger.InfoCtx(ctx, "Evaluated scenario", "score", score, "outcome", o)

	return se, nil
}
