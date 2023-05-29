package dto

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
)

type NodeDto struct {
	FuncName      string             `json:"funcName,omitempty"`
	Constant      any                `json:"constant,omitempty"`
	Children      []NodeDto          `json:"children,omitempty"`
	NamedChildren map[string]NodeDto `json:"named_children,omitempty"`
}

func AdaptNodeDto(node ast.Node) (NodeDto, error) {

	funcName, err := adaptDtoFunctionName(node.Function)
	if err != nil {
		return NodeDto{}, err
	}

	childrenDto, err := utils.MapErr(node.Children, AdaptNodeDto)
	if err != nil {
		return NodeDto{}, err
	}

	namedChildrenDto, err := utils.MapMapErr(node.NamedChildren, AdaptNodeDto)
	if err != nil {
		return NodeDto{}, err
	}

	return NodeDto{
		FuncName:      funcName,
		Constant:      node.Constant,
		Children:      childrenDto,
		NamedChildren: namedChildrenDto,
	}, nil
}

func adaptDtoFunctionName(f ast.Function) (string, error) {
	switch f {
	case ast.FUNC_CONSTANT:
		return "", nil
	case ast.FUNC_PLUS:
		return "+", nil
	case ast.FUNC_MINUS:
		return "-", nil
	case ast.FUNC_GREATER:
		return ">", nil
	case ast.FUNC_LESS:
		return "<", nil
	case ast.FUNC_EQUAL:
		return "=", nil
	case ast.FUNC_READ_PAYLOAD:
		return "ReadPayload", nil
	case ast.FUNC_DB_ACCESS:
		return "DatabaseAccess", nil
	default:
		return "", fmt.Errorf("function not supported by json renderer: %s", f.DebugString())
	}
}

func AdaptASTNode(dto NodeDto) (ast.Node, error) {

	function, err := adaptFunctionName(dto.FuncName)
	if err != nil {
		return ast.Node{}, err
	}

	children, err := utils.MapErr(dto.Children, AdaptASTNode)
	if err != nil {
		return ast.Node{}, err
	}

	namedChildren, err := utils.MapMapErr(dto.NamedChildren, AdaptASTNode)
	if err != nil {
		return ast.Node{}, err
	}

	return ast.Node{
		Function:      function,
		Constant:      dto.Constant,
		Children:      children,
		NamedChildren: namedChildren,
	}, nil
}

func adaptFunctionName(f string) (ast.Function, error) {

	switch f {
	case "":
		return ast.FUNC_CONSTANT, nil
	case "+":
		return ast.FUNC_PLUS, nil
	case "-":
		return ast.FUNC_MINUS, nil
	case ">":
		return ast.FUNC_GREATER, nil
	case "<":
		return ast.FUNC_LESS, nil
	case "=":
		return ast.FUNC_EQUAL, nil
	case "ReadPayload":
		return ast.FUNC_READ_PAYLOAD, nil
	case "DatabaseAccess":
		return ast.FUNC_DB_ACCESS, nil
	default:
		return -1, fmt.Errorf("function not supported by json renderer: %s", f)
	}
}
