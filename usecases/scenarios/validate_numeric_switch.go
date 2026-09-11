package scenarios

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/cockroachdb/errors"
)

// Structural validation for numeric-first-dimension Switches stored in rule formulas.
//
// Evaluation is first-match on boolean Case predicates (see evaluate.Switch). Overlapping
// `<=` thresholds become ranges only if Cases are ordered and homogeneous. This file does
// not change eval: it walks the unevaluated AST at publish / dry-run time.
//
// Stored shapes this validates:
//   - 1-variable: Case(<=(F, t), returnValue)
//   - matrix:     Case(And(<=(F, t), =(G, v)), returnValue)
//
// String / risk-level / two-string switches and scoring Switches (ScoreComputation
// children) do not match those shapes and are left alone.
//
// Called from ValidateScenarioAstImpl and ValidateScenarioIterationImpl via
// appendNumericSwitchValidationErrors.

type numericSwitchCaseKind int

const (
	numericSwitchCaseOther numericSwitchCaseKind = iota
	numericSwitchCaseOneVar
	numericSwitchCaseMatrix
)

// classifiedNumericSwitchCase is the parsed first-dimension (and optional second-dimension)
// identity of one Case, used to compare fields and threshold order without re-evaluating.
type classifiedNumericSwitchCase struct {
	kind       numericSwitchCaseKind
	fieldHash  uint64  // hash of F, the left operand of `<=`
	secondHash uint64  // hash of G, the left operand of `=`; matrix only
	threshold  float64 // t, the constant right operand of `<=`
}

// validateNumericSwitchesInAst collects NumericSwitchInvalid errors for every Switch
// nested in node, including under Or / And / comparisons. Used by tests and by
// appendNumericSwitchValidationErrors.
func validateNumericSwitchesInAst(node ast.Node) []models.ScenarioValidationError {
	var errs []models.ScenarioValidationError
	walkAstForNumericSwitches(node, &errs)
	return errs
}

// walkAstForNumericSwitches depth-first visits positional and named children so a Switch
// used as a numeric operand (e.g. `>(Switch, 5)`) is still validated.
func walkAstForNumericSwitches(node ast.Node, errs *[]models.ScenarioValidationError) {
	if node.Function == ast.FUNC_SWITCH {
		*errs = append(*errs, validateNumericSwitch(node)...)
	}
	for _, child := range node.Children {
		walkAstForNumericSwitches(child, errs)
	}
	for _, child := range node.NamedChildren {
		walkAstForNumericSwitches(child, errs)
	}
}

// validateNumericSwitch inspects one Switch. It returns immediately when the node is not a
// Case-only switch (scoring) or when every Case is a non-numeric boolean predicate.
//
// Otherwise all Cases must share one shape (1-var xor matrix). Then:
//   - every `<=` uses the same field F
//   - matrix: every `=` uses the same field G
//   - thresholds t are non-decreasing in Case order (equal t allowed for matrix columns)
func validateNumericSwitch(node ast.Node) []models.ScenarioValidationError {
	if len(node.Children) == 0 {
		return nil
	}
	for _, child := range node.Children {
		if child.Function != ast.FUNC_CASE {
			return nil
		}
	}

	cases := make([]classifiedNumericSwitchCase, 0, len(node.Children))
	nOneVar, nMatrix, nOther := 0, 0, 0
	for _, child := range node.Children {
		classified := classifyNumericSwitchCase(child)
		cases = append(cases, classified)
		switch classified.kind {
		case numericSwitchCaseOneVar:
			nOneVar++
		case numericSwitchCaseMatrix:
			nMatrix++
		default:
			nOther++
		}
	}

	if nOther == len(cases) {
		return nil
	}
	if nOther > 0 || (nOneVar > 0 && nMatrix > 0) {
		return []models.ScenarioValidationError{numericSwitchError(
			"numeric switch mixes incompatible case predicates")}
	}

	var errs []models.ScenarioValidationError
	fieldHash := cases[0].fieldHash
	for _, c := range cases {
		if c.fieldHash != fieldHash {
			errs = append(errs, numericSwitchError(
				"numeric switch cases must use the same field on the first dimension"))
			break
		}
	}

	if nMatrix == len(cases) {
		secondHash := cases[0].secondHash
		for _, c := range cases {
			if c.secondHash != secondHash {
				errs = append(errs, numericSwitchError(
					"numeric switch matrix cases must use the same field on the second dimension"))
				break
			}
		}
	}

	for i := 1; i < len(cases); i++ {
		if cases[i].threshold < cases[i-1].threshold {
			errs = append(errs, numericSwitchError(
				"numeric switch thresholds must be non-decreasing"))
			break
		}
	}

	return errs
}

// classifyNumericSwitchCase looks only at Case child 0 (the predicate). Child 1 is the
// numeric return value and is checked at evaluation time, not here.
//
// Matching is exact: a lone `<=(F, t)` is 1-var; `And` must be `<=` then `=` with two
// children each. Swapped And operands, `<`, or a non-constant t are "other" and do not
// enter numeric-first-dimension mode unless mixed with matching Cases (then a mix error).
func classifyNumericSwitchCase(caseNode ast.Node) classifiedNumericSwitchCase {
	if len(caseNode.Children) == 0 {
		return classifiedNumericSwitchCase{kind: numericSwitchCaseOther}
	}

	predicate := caseNode.Children[0]
	if fieldHash, threshold, ok := parseNumericLessOrEqual(predicate); ok {
		return classifiedNumericSwitchCase{
			kind:      numericSwitchCaseOneVar,
			fieldHash: fieldHash,
			threshold: threshold,
		}
	}

	if predicate.Function != ast.FUNC_AND || len(predicate.Children) != 2 {
		return classifiedNumericSwitchCase{kind: numericSwitchCaseOther}
	}

	fieldHash, threshold, ok := parseNumericLessOrEqual(predicate.Children[0])
	if !ok {
		return classifiedNumericSwitchCase{kind: numericSwitchCaseOther}
	}
	second := predicate.Children[1]
	if second.Function != ast.FUNC_EQUAL || len(second.Children) != 2 {
		return classifiedNumericSwitchCase{kind: numericSwitchCaseOther}
	}

	return classifiedNumericSwitchCase{
		kind:       numericSwitchCaseMatrix,
		fieldHash:  fieldHash,
		secondHash: second.Children[0].Hash(),
		threshold:  threshold,
	}
}

// parseNumericLessOrEqual accepts `<=(F, t)` where t is a numeric constant.
// F is identified by Node.Hash so Payload("amount") compares equal across Cases
// without requiring pointer identity.
func parseNumericLessOrEqual(node ast.Node) (uint64, float64, bool) {
	if node.Function != ast.FUNC_LESS_OR_EQUAL || len(node.Children) != 2 {
		return 0, 0, false
	}
	threshold, ok := numericConstant(node.Children[1])
	if !ok {
		return 0, 0, false
	}
	return node.Children[0].Hash(), threshold, true
}

// numericConstant reads an int/float FUNC_CONSTANT. JSON-stored formulas typically
// unmarshal numbers as float64; Go-built tests may use int.
func numericConstant(node ast.Node) (float64, bool) {
	if node.Function != ast.FUNC_CONSTANT {
		return 0, false
	}
	value, err := evaluate.ToFloat64(node.Constant)
	if err != nil {
		return 0, false
	}
	return value, true
}

func numericSwitchError(message string) models.ScenarioValidationError {
	return models.ScenarioValidationError{
		Error: errors.Wrap(models.BadParameterError, message),
		Code:  models.NumericSwitchInvalid,
	}
}

// appendNumericSwitchValidationErrors is the hook used by scenario AST / iteration
// validation. Nil formulas (missing rule AST) are skipped.
func appendNumericSwitchValidationErrors(node *ast.Node, dest *[]models.ScenarioValidationError) {
	if node == nil {
		return
	}
	*dest = append(*dest, validateNumericSwitchesInAst(*node)...)
}
