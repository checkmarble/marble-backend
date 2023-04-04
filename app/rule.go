package app

import "marble/marble-backend/app/operators"

///////////////////////////////
// Rule
///////////////////////////////

type Rule struct {
	DisplayOrder int
	Name         string
	Description  string

	Formula       operators.OperatorBool
	ScoreModifier int
}

///////////////////////////////
// Rule Execution
///////////////////////////////

type RuleExecution struct {
	Rule                Rule
	Result              bool
	ResultScoreModifier int
	Error               RuleExecutionError
}

///////////////////////////////
// RuleExecutionError
///////////////////////////////

type RuleExecutionError int

const (
	FieldEmptyOrMissing RuleExecutionError = 200
)

func (r RuleExecutionError) String() string {
	switch r {
	case FieldEmptyOrMissing:
		return "A field in rule is empty or missing"
	}
	return ""
}

///////////////////////////////
//
///////////////////////////////

func (r Rule) Eval(dataAccessor operators.DataAccessor) RuleExecution {

	// Eval the Node
	res := r.Formula.Eval(dataAccessor)

	score := 0
	if res {
		score = r.ScoreModifier
	}

	re := RuleExecution{
		Rule:                r,
		Result:              res,
		ResultScoreModifier: score,
		// TODO error ?
	}

	//log.Printf("Rule %s is %v, score = %v", r.RootNode.Print(p), res, score)

	return re
}
