package app

import (
	"errors"
	"log"
	"marble/marble-backend/app/operators"
	"runtime/debug"
	"time"
)

///////////////////////////////
// Scenario
///////////////////////////////

// type Scenario struct {
// 	ID string

// 	Name        string
// 	Description string

// 	Version string

// 	TriggerCondition    Node   // A trigger condition is a formula tree
// 	Rules               []Rule // Rules have a formula + score + metadata
// 	TriggerObjectType   string
// 	OutcomeApproveScore int
// 	OutcomeRejectScore  int
// }

type Scenario struct {
	ID string

	Name        string
	Description string

	TriggerObjectType string

	CreatedAt   time.Time
	LiveVersion *ScenarioIteration
}

type ScenarioIteration struct {
	ID         string
	ScenarioID string
	Version    int

	CreatedAt time.Time
	UpdatedAt time.Time

	Body ScenarioIterationBody
}

type ScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	Rules                []Rule
	ScoreReviewThreshold int
	ScoreRejectThreshold int
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

func (s Scenario) Eval(p Payload) (se ScenarioExecution, err error) {

	///////////////////////////////
	// Recover in case the evaluation panicked.
	// Even if there is a "recoverer" middleware in our stack, this allows a sentinel error to be used and to catch the failure early
	///////////////////////////////
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic: %v", r)

			log.Println("stacktrace from panic: ")
			log.Println(string(debug.Stack()))

			err = ErrPanicInScenarioEvalution
			se = ScenarioExecution{}
		}
	}()

	log.Printf("Evaluating scenario %s", s.ID)

	// If the scenario has no live version, don't try to Eval() it, return early
	if s.LiveVersion == nil {
		return ScenarioExecution{}, ErrScenarioHasNoLiveVersion
	}

	// Check the scenario & trigger_object's types
	if s.TriggerObjectType != p.TableName {
		return ScenarioExecution{}, ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch
	}

	// Evaluate the trigger
	triggerPassed := s.LiveVersion.Body.TriggerCondition.Eval()

	if !triggerPassed {
		return ScenarioExecution{}, ErrScenarioTriggerConditionAndTriggerObjectMismatch
	}

	// Evaluate all rules
	score := 0
	ruleExecutions := make([]RuleExecution, 0)
	for _, rule := range s.LiveVersion.Body.Rules {

		// Evaluate single rule
		ruleExecution := rule.Eval(p)
		log.Printf("Rule %s (score_modifier = %v) is %v\n", ruleExecution.Rule.Formula.Print(), ruleExecution.Rule.ScoreModifier, ruleExecution.Result)

		// Increment scenario score when rule is true
		if ruleExecution.Result {
			score += ruleExecution.Rule.ScoreModifier
		}

		ruleExecutions = append(ruleExecutions, ruleExecution)
	}

	// Compute outcome from score
	o := None

	if score < s.LiveVersion.Body.ScoreReviewThreshold {
		o = Approve
	}
	if score >= s.LiveVersion.Body.ScoreReviewThreshold && score < s.LiveVersion.Body.ScoreRejectThreshold {
		o = Review
	}
	if score > s.LiveVersion.Body.ScoreRejectThreshold {
		o = Reject
	}

	// Build ScenarioExecution as result
	se = ScenarioExecution{
		ScenarioID:          s.ID,
		ScenarioName:        s.Name,
		ScenarioDescription: s.Description,
		ScenarioVersion:     s.LiveVersion.Version,
		RuleExecutions:      ruleExecutions,
		Score:               score,
		Outcome:             o,
	}

	log.Printf("scenario %s (Rev:%v Rej:%v), score = %v, outcome = %s", s.ID, s.LiveVersion.Body.ScoreReviewThreshold, s.LiveVersion.Body.ScoreRejectThreshold, score, o.String())

	return se, nil
}
