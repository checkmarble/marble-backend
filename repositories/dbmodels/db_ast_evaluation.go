package dbmodels

import (
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type dbNodeEvaluation struct {
	ReturnValue   any                         `json:"return_value"`
	ErrorCodes    []models.ExecutionError     `json:"error_codes"`
	Children      []dbNodeEvaluation          `json:"children,omitempty"`
	NamedChildren map[string]dbNodeEvaluation `json:"named_children,omitempty"`
}

func adaptDbNodeEvaluation(nodeEvaluation ast.NodeEvaluation) (dbNodeEvaluation, error) {
	childrenDb, err := pure_utils.MapErr(nodeEvaluation.Children, adaptDbNodeEvaluation)
	if err != nil {
		return dbNodeEvaluation{}, err
	}

	namedChildrenDb, err := pure_utils.MapValuesErr(nodeEvaluation.NamedChildren, adaptDbNodeEvaluation)
	if err != nil {
		return dbNodeEvaluation{}, err
	}

	return dbNodeEvaluation{
		ReturnValue:   nodeEvaluation.ReturnValue,
		ErrorCodes:    pure_utils.Map(nodeEvaluation.Errors, models.AdaptExecutionError),
		Children:      childrenDb,
		NamedChildren: namedChildrenDb,
	}, nil
}

func adaptNodeEvaluation(dbNodeEvaluation dbNodeEvaluation) (ast.NodeEvaluation, error) {
	children, err := pure_utils.MapErr(dbNodeEvaluation.Children, adaptNodeEvaluation)
	if err != nil {
		return ast.NodeEvaluation{}, err
	}

	namedChildren, err := pure_utils.MapValuesErr(dbNodeEvaluation.NamedChildren, adaptNodeEvaluation)
	if err != nil {
		return ast.NodeEvaluation{}, err
	}

	return ast.NodeEvaluation{
		ReturnValue:   dbNodeEvaluation.ReturnValue,
		Errors:        pure_utils.Map(dbNodeEvaluation.ErrorCodes, adaptErrorCodeAsError),
		Children:      children,
		NamedChildren: namedChildren,
	}, nil
}

func SerializeNodeEvaluation(nodeEvaluation *ast.NodeEvaluation) ([]byte, error) {
	if nodeEvaluation == nil {
		return nil, nil
	}

	dbNodeEvaluation, err := adaptDbNodeEvaluation(*nodeEvaluation)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal rule formula ast expression: %w", err)
	}

	return json.Marshal(dbNodeEvaluation)
}

func DeserializeNodeEvaluation(serializedNodeEvaluation []byte) (*ast.NodeEvaluation, error) {
	if len(serializedNodeEvaluation) == 0 {
		return nil, nil
	}

	var dbNodeEvaluation dbNodeEvaluation
	if err := json.Unmarshal(serializedNodeEvaluation, &dbNodeEvaluation); err != nil {
		return nil, err
	}

	nodeEvaluation, err := adaptNodeEvaluation(dbNodeEvaluation)
	if err != nil {
		return nil, err
	}
	return &nodeEvaluation, nil
}
