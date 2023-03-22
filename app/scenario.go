package app

import (
	"errors"
	"fmt"
	"log"
	"runtime/debug"
)

///////////////////////////////
// Scenario
///////////////////////////////

type Scenario struct {
	ID string

	Name        string
	Description string

	Version string

	TriggerCondition    Node   // A trigger condition is a formula tree
	Rules               []Rule // Rules have a formula + score + metadata
	TriggerObjectType   string
	OutcomeApproveScore int
	OutcomeRejectScore  int
}

///////////////////////////////
// ScenarioExecution
///////////////////////////////

type ScenarioExecution struct {
	Scenario       Scenario
	RuleExecutions []RuleExecution
	Score          int
	Outcome        Outcome
}

var (
	ErrPanicInScenarioEvalution                         = errors.New("panic during scenario evaluation")
	ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch   = errors.New("scenario's trigger_type and provided trigger_object type are different")
	ErrScenarioTriggerConditionAndTriggerObjectMismatch = errors.New("trigger_object does not match the scenario's trigger conditions")
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

	// Check the scenario & trigger_object's types
	if s.TriggerObjectType != p.TableName {
		return ScenarioExecution{}, ErrScenarioTriggerConditionAndTriggerObjectMismatch
	}

	// Evaluate the trigger
	triggerPassed, ok := s.TriggerCondition.Eval(p).(bool)
	if !ok {
		return ScenarioExecution{}, fmt.Errorf("unable to evaluate trigger condition")
	}

	if !triggerPassed {
		return ScenarioExecution{}, ErrScenarioTriggerConditionAndTriggerObjectMismatch
	}

	// Evaluate all rules
	score := 0
	ruleExecutions := make([]RuleExecution, 0)
	for _, rule := range s.Rules {

		// Evaluate single rule
		ruleExecution := rule.Eval(p)
		log.Printf("Rule %s (score_modifier = %v) is %v\n", ruleExecution.Rule.RootNode.Print(p), ruleExecution.Rule.ScoreModifier, ruleExecution.Result)

		// Increment scenario score when rule is true
		if ruleExecution.Result {
			score += ruleExecution.Rule.ScoreModifier
		}

		ruleExecutions = append(ruleExecutions, ruleExecution)
	}

	// Compute outcome from score
	o := None

	if score < s.OutcomeApproveScore {
		o = Approve
	}
	if score >= s.OutcomeApproveScore && score < s.OutcomeRejectScore {
		o = Review
	}
	if score >= s.OutcomeRejectScore {
		o = Approve
	}

	// Build ScenarioExecution as result
	se = ScenarioExecution{
		Scenario:       s,
		RuleExecutions: ruleExecutions,
		Score:          score,
		Outcome:        o,
	}

	log.Printf("scenario %s (A:%v R:%v), score = %v, outcome = %s", s.ID, s.OutcomeApproveScore, s.OutcomeRejectScore, score, o.String())

	return se, nil
}
