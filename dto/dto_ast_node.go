package dto

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

type NodeDto struct {
	FuncName      string             `json:"name,omitempty"`
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

	namedChildrenDto, err := utils.MapValuesErr(node.NamedChildren, AdaptNodeDto)
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

	attributes, err := f.Attributes()
	return attributes.AstName, err
}

func AdaptASTNode(dto NodeDto) (ast.Node, error) {

	if dto.FuncName == "Unknown" {
		dto.FuncName = "Undefined"
	}

	function, err := adaptFunctionName(dto.FuncName)
	if err != nil {
		return ast.Node{}, err
	}

	children, err := utils.MapErr(dto.Children, AdaptASTNode)
	if err != nil {
		return ast.Node{}, err
	}

	namedChildren, err := utils.MapValuesErr(dto.NamedChildren, AdaptASTNode)
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

var astNameFuncMap = func() map[string]ast.Function {
	result := make(map[string]ast.Function, len(ast.FuncAttributesMap))
	for f, attributes := range ast.FuncAttributesMap {
		result[attributes.AstName] = f
	}
	return result
}()

func adaptFunctionName(f string) (ast.Function, error) {

	if f, ok := astNameFuncMap[f]; ok {
		return f, nil
	}

	return ast.FUNC_UNKNOWN, fmt.Errorf("unknown function: %v", f)
}
