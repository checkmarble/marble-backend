package dbmodels

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models/ast"
)

func SerializeNodeEvaluationDto(nodeEvaluation *ast.NodeEvaluationDto) ([]byte, error) {
	if nodeEvaluation == nil {
		return nil, nil
	}

	return json.Marshal(&nodeEvaluation)
}

func SerializeDecisionEvaluationDto(decisionEvaluation []*ast.NodeEvaluationDto) ([]byte, error) {
	if decisionEvaluation == nil {
		return nil, nil
	}

	return json.Marshal(&decisionEvaluation)
}

func DeserializeNodeEvaluationDto(serializedNodeEvaluationDto []byte) (*ast.NodeEvaluationDto, error) {
	if len(serializedNodeEvaluationDto) == 0 {
		return nil, nil
	}

	var nodeEvaluationDto ast.NodeEvaluationDto
	if err := json.Unmarshal(serializedNodeEvaluationDto, &nodeEvaluationDto); err != nil {
		return nil, err
	}

	return &nodeEvaluationDto, nil
}

func DeserializeDecisionEvaluationDto(serializedDecisionEvaluationDto []byte) ([]*ast.NodeEvaluationDto, error) {
	if len(serializedDecisionEvaluationDto) == 0 {
		return nil, nil
	}

	var nodeEvaluationDto []*ast.NodeEvaluationDto

	if err := json.Unmarshal(serializedDecisionEvaluationDto, &nodeEvaluationDto); err != nil {
		return nil, err
	}

	return nodeEvaluationDto, nil
}
