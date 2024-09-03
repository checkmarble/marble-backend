package ast

import "github.com/checkmarble/marble-backend/pure_utils"

// When too long, the ReturnValue can be omitted (ex: when function is `FUNC_CUSTOM_LIST_ACCESS“).
// In such cases, ReturnValue will be nil and Omitted will be true.
type ReturnValueDto struct {
	Value     any  `json:"value,omitempty"`
	IsOmitted bool `json:"is_omitted"`
}

type NodeEvaluationDto struct {
	ReturnValue   ReturnValueDto               `json:"return_value"`
	Errors        []EvaluationErrorDto         `json:"errors"`
	Children      []NodeEvaluationDto          `json:"children,omitempty"`
	NamedChildren map[string]NodeEvaluationDto `json:"named_children,omitempty"`
}

func AdaptNodeEvaluationDto(evaluation NodeEvaluation) NodeEvaluationDto {
	var returnValueDto ReturnValueDto
	if isReturnValueOmitted(evaluation) {
		returnValueDto = ReturnValueDto{Value: nil, IsOmitted: true}
	} else {
		returnValueDto = ReturnValueDto{Value: evaluation.ReturnValue, IsOmitted: false}
	}

	return NodeEvaluationDto{
		ReturnValue:   returnValueDto,
		Errors:        pure_utils.Map(evaluation.Errors, AdaptEvaluationErrorDto),
		Children:      pure_utils.Map(evaluation.Children, AdaptNodeEvaluationDto),
		NamedChildren: pure_utils.MapValues(evaluation.NamedChildren, AdaptNodeEvaluationDto),
	}
}

func isReturnValueOmitted(evaluation NodeEvaluation) bool {
	return evaluation.Function == FUNC_CUSTOM_LIST_ACCESS
}
