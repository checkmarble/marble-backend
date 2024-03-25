package ast

import "github.com/checkmarble/marble-backend/pure_utils"

type NodeEvaluationDto struct {
	// When too long, the ReturnValue can be omitted (ex: when function is `FUNC_CUSTOM_LIST_ACCESS``).
	// In such cases, ReturnValue will be the constant `omittedReturnValue``
	ReturnValue   any                          `json:"return_value"`
	Errors        []EvaluationErrorDto         `json:"errors"`
	Children      []NodeEvaluationDto          `json:"children,omitempty"`
	NamedChildren map[string]NodeEvaluationDto `json:"named_children,omitempty"`
}

func AdaptNodeEvaluationDto(evaluation NodeEvaluation) NodeEvaluationDto {
	nodeEvaluationDto := NodeEvaluationDto{
		ReturnValue:   evaluation.ReturnValue,
		Errors:        pure_utils.Map(evaluation.Errors, AdaptEvaluationErrorDto),
		Children:      pure_utils.Map(evaluation.Children, AdaptNodeEvaluationDto),
		NamedChildren: pure_utils.MapValues(evaluation.NamedChildren, AdaptNodeEvaluationDto),
	}

	if isReturnValueOmitted(evaluation) {
		nodeEvaluationDto.ReturnValue = omittedReturnValue
	}

	return nodeEvaluationDto
}

const omittedReturnValue = "__omitted_return_value"

func isReturnValueOmitted(evaluation NodeEvaluation) bool {
	return evaluation.Function == FUNC_CUSTOM_LIST_ACCESS
}
