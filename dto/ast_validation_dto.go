package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type AstValidationDto struct {
	Errors     []ScenarioValidationErrorDto `json:"errors"`
	Evaluation ast.NodeEvaluationDto        `json:"evaluation"`
}

func AdaptAstValidationDto(s models.AstValidation) AstValidationDto {
	return AstValidationDto{
		Errors:     pure_utils.Map(s.Errors, AdaptScenarioValidationErrorDto),
		Evaluation: ast.AdaptNodeEvaluationDto(s.Evaluation),
	}
}
