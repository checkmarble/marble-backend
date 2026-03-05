package scoring

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type ScoringEvaluation struct {
	Modifier    int                     `json:"modifier"`
	Floor       int                     `json:"floor"`
	RiskLevel   int                     `json:"risk_level"`
	Evaluations []ast.NodeEvaluationDto `json:"evaluations"`
}

func AdaptScoringEvaluation(m models.ScoringEvaluation) ScoringEvaluation {
	eval := ScoringEvaluation{
		Modifier:    m.Modifier,
		Floor:       m.Floor,
		RiskLevel:   m.RiskLevel,
		Evaluations: pure_utils.Map(m.Evaluation, ast.AdaptNodeEvaluationDto),
	}

	return eval
}
