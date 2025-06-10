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
	Skipped       bool                         `json:"skipped"`
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
		Skipped:       evaluation.EvaluationPlan.Skipped,
	}
}

func isReturnValueOmitted(evaluation NodeEvaluation) bool {
	return evaluation.Function == FUNC_CUSTOM_LIST_ACCESS
}

func MergeAstTrees(definition Node, evaluation NodeEvaluationDto) NodeEvaluationWithDefinitionDto {
	attrs, _ := definition.Function.Attributes()

	out := NodeEvaluationWithDefinitionDto{
		ReturnValue: evaluation.ReturnValue,
		Errors:      evaluation.Errors,
		Skipped:     evaluation.Skipped,
		Function:    attrs.AstName,
		Constant:    definition.Constant,
	}

	out.Children = make([]NodeEvaluationWithDefinitionDto, len(evaluation.Children))
	for i, child := range evaluation.Children {
		out.Children[i] = MergeAstTrees(definition.Children[i], child)
	}

	out.NamedChildren = make(map[string]NodeEvaluationWithDefinitionDto, len(evaluation.NamedChildren))
	for name, child := range evaluation.NamedChildren {
		out.NamedChildren[name] = MergeAstTrees(definition.NamedChildren[name], child)
	}

	if definition.Constant != nil {
		out.Constant = definition.Constant
	}

	return out
}

type NodeEvaluationWithDefinitionDto struct {
	ReturnValue   ReturnValueDto                             `json:"return_value"`
	Errors        []EvaluationErrorDto                       `json:"errors,omitempty"`
	Children      []NodeEvaluationWithDefinitionDto          `json:"children,omitempty"`
	NamedChildren map[string]NodeEvaluationWithDefinitionDto `json:"named_children,omitempty"`
	Skipped       bool                                       `json:"skipped"`
	Function      string                                     `json:"function"`
	Constant      any                                        `json:"constant,omitempty"`
}
