package app

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app/operators"
	"marble/marble-backend/models"
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

	err := si.IsValidForPublication()
	if err != nil {
		return PublishedScenarioIteration{}, err
	}

	result.Version = *si.Version
	result.Body.ScoreReviewThreshold = *si.Body.ScoreReviewThreshold
	result.Body.ScoreRejectThreshold = *si.Body.ScoreRejectThreshold
	result.Body.Rules = si.Body.Rules
	result.Body.TriggerCondition = si.Body.TriggerCondition

	return result, nil
}

func (si ScenarioIteration) IsValidForPublication() error {
	if si.Body.ScoreReviewThreshold == nil {
		return fmt.Errorf("Scenario iteration has no ScoreReviewThreshold: \n%w", ErrScenarioIterationNotValid)
	}

	if si.Body.ScoreRejectThreshold == nil {
		return fmt.Errorf("Scenario iteration has no ScoreRejectThreshold: \n%w", ErrScenarioIterationNotValid)
	}

	if len(si.Body.Rules) < 1 {
		return fmt.Errorf("Scenario iteration has no rules: \n%w", ErrScenarioIterationNotValid)
	}
	for _, rule := range si.Body.Rules {
		if !rule.Formula.IsValid() {
			return fmt.Errorf("Scenario iteration rule has invalid rules: \n%w", ErrScenarioIterationNotValid)
		}
	}

	if si.Body.TriggerCondition == nil {
		return fmt.Errorf("Scenario iteration has no trigger condition: \n%w", ErrScenarioIterationNotValid)
	} else if !si.Body.TriggerCondition.IsValid() {
		return fmt.Errorf("Scenario iteration trigger condition is invalid: \n%w", ErrScenarioIterationNotValid)
	}

	return nil
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

type ClientDataRepositoryItf interface {
	GetDbField(ctx context.Context, readParams models.DbFieldReadParams) (interface{}, error)
}

func (s Scenario) Eval(ctx context.Context, repo RepositoryInterface, payloadStructWithReader models.Payload, dataModel models.DataModel, logger *slog.Logger) (se ScenarioExecution, err error) {

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

	orgID, err := utils.OrgIDFromCtx(ctx, nil)
	if err != nil {
		return ScenarioExecution{}, err
	}
	liveVersion, err := repo.GetScenarioIteration(ctx, orgID, *s.LiveVersionID)
	if err != nil {
		return ScenarioExecution{}, err
	}

	publishedVersion, err := NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return ScenarioExecution{}, err
	}

	// Check the scenario & trigger_object's types
	if s.TriggerObjectType != string(payloadStructWithReader.Table.Name) {
		return ScenarioExecution{}, ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch
	}

	dataAccessor := DataAccessorImpl{DataModel: dataModel, Payload: payloadStructWithReader, repository: repo}

	// Evaluate the trigger
	triggerPassed, err := publishedVersion.Body.TriggerCondition.Eval(ctx, &dataAccessor)
	if err != nil {
		return ScenarioExecution{}, err
	}

	if !triggerPassed {
		return ScenarioExecution{}, ErrScenarioTriggerConditionAndTriggerObjectMismatch
	}

	// Evaluate all rules
	score := 0
	ruleExecutions := make([]RuleExecution, 0)
	for _, rule := range publishedVersion.Body.Rules {
		scoreModifier, ruleExecution, err := evalScenarioRule(ctx, rule, &dataAccessor, logger)
		if err != nil {
			return ScenarioExecution{}, err
		}
		score += scoreModifier
		ruleExecutions = append(ruleExecutions, ruleExecution)
	}

	// Compute outcome from score
	o := None

	if score < publishedVersion.Body.ScoreReviewThreshold {
		o = Approve
	}
	if score >= publishedVersion.Body.ScoreReviewThreshold && score < publishedVersion.Body.ScoreRejectThreshold {
		o = Review
	}
	if score > publishedVersion.Body.ScoreRejectThreshold {
		o = Reject
	}

	// Build ScenarioExecution as result
	se = ScenarioExecution{
		ScenarioID:          s.ID,
		ScenarioName:        s.Name,
		ScenarioDescription: s.Description,
		ScenarioVersion:     publishedVersion.Version,
		RuleExecutions:      ruleExecutions,
		Score:               score,
		Outcome:             o,
	}

	logger.InfoCtx(ctx, "Evaluated scenario", "score", score, "outcome", o)

	return se, nil
}

func evalScenarioRule(ctx context.Context, rule Rule, dataAccessor operators.DataAccessor, logger *slog.Logger) (int, RuleExecution, error) {
	// Evaluate single rule
	score := 0
	ruleExecution, err := rule.Eval(ctx, dataAccessor)
	if err != nil {
		ruleExecution.Rule = rule
		ruleExecution, err = setRuleExecutionError(ruleExecution, err)
		if err != nil {
			return score, ruleExecution, err
		}
		logger.InfoCtx(ctx, "Rule had an error",
			slog.String("ruleName", rule.Name),
			slog.String("ruleId", rule.ID),
			slog.String("formula", rule.Formula.String()),
			slog.String("error", ruleExecution.Error.String()),
		)
	}

	// Increment scenario score when rule is true
	if ruleExecution.Result {
		logger.InfoCtx(ctx, "Rule executed",
			slog.Int("score_modifier", rule.ScoreModifier),
			slog.String("ruleName", rule.Name),
			slog.Bool("result", ruleExecution.Result),
		)
		fmt.Printf("rule score modifier: %d\n", ruleExecution.Rule.ScoreModifier)
		score = ruleExecution.Rule.ScoreModifier
	}
	return score, ruleExecution, nil
}

func setRuleExecutionError(ruleExecution RuleExecution, err error) (RuleExecution, error) {
	if errors.Is(err, models.OperatorNullValueReadError) {
		ruleExecution.Error = NullFieldRead
	} else if errors.Is(err, models.OperatorDivisionByZeroError) {
		ruleExecution.Error = DivisionByZero
	} else if errors.Is(err, models.OperatorNoRowsReadInDbError) {
		ruleExecution.Error = NoRowsRead
	} else {
		// return early in case of an unexpected error
		return ruleExecution, err
	}
	return ruleExecution, nil
}
