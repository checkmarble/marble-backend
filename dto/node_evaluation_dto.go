package dto

import (
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
)

type NodeEvaluationDto struct {
	ReturnValue     any    `json:"return_value,omitempty"`
	EvaluationError string `json:"evaluation_error,omitempty"`

	Children      []NodeEvaluationDto          `json:"children,omitempty"`
	NamedChildren map[string]NodeEvaluationDto `json:"named_children,omitempty"`
}

func AdaptNodeEvaluationDto(nodeError ast.NodeEvaluation) NodeEvaluationDto {

	evaluationError := ""
	if nodeError.EvaluationError != nil {
		evaluationError = nodeError.EvaluationError.Error()
	}
	return NodeEvaluationDto{
		ReturnValue:     nodeError.ReturnValue,
		EvaluationError: evaluationError,
		Children:        utils.Map(nodeError.Children, AdaptNodeEvaluationDto),
		NamedChildren:   utils.MapMap(nodeError.NamedChildren, AdaptNodeEvaluationDto),
	}
}
