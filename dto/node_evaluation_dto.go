package dto

import (
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
)

type NodeEvaluationDto struct {
	ReturnValue any                  `json:"return_value,omitempty"`
	Errors      []EvaluationErrorDto `json:"errors,omitempty"`

	Children      []NodeEvaluationDto          `json:"children,omitempty"`
	NamedChildren map[string]NodeEvaluationDto `json:"named_children,omitempty"`
}

func AdaptNodeEvaluationDto(evaluation ast.NodeEvaluation) NodeEvaluationDto {

	return NodeEvaluationDto{
		ReturnValue:   evaluation.ReturnValue,
		Errors:        utils.Map(evaluation.Errors, AdaptEvaluationErrorDto),
		Children:      utils.Map(evaluation.Children, AdaptNodeEvaluationDto),
		NamedChildren: utils.MapMap(evaluation.NamedChildren, AdaptNodeEvaluationDto),
	}
}
