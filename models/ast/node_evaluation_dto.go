package ast

import "github.com/checkmarble/marble-backend/pure_utils"

type NodeEvaluationDto struct {
	ReturnValue   any                          `json:"return_value"`
	Errors        []EvaluationErrorDto         `json:"errors"`
	Children      []NodeEvaluationDto          `json:"children,omitempty"`
	NamedChildren map[string]NodeEvaluationDto `json:"named_children,omitempty"`
}

func AdaptNodeEvaluationDto(evaluation NodeEvaluation) NodeEvaluationDto {
	return NodeEvaluationDto{
		ReturnValue:   evaluation.ReturnValue,
		Errors:        pure_utils.Map(evaluation.Errors, AdaptEvaluationErrorDto),
		Children:      pure_utils.Map(evaluation.Children, AdaptNodeEvaluationDto),
		NamedChildren: pure_utils.MapValues(evaluation.NamedChildren, AdaptNodeEvaluationDto),
	}
}
