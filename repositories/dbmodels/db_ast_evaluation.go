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
