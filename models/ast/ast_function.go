package ast

import (
	"fmt"
)

type Function int

const (
	FUNC_CONSTANT Function = iota
	FUNC_ADD
	FUNC_SUBTRACT
	FUNC_MULTIPLY
	FUNC_DIVIDE
	FUNC_GREATER
	FUNC_LESS
	FUNC_EQUAL
	FUNC_NOT
	FUNC_AND
	FUNC_OR
	FUNC_VARIABLE
	FUNC_DB_ACCESS
	FUNC_CUSTOM_LIST_ACCESS
	FUNC_IS_IN_LIST
	FUNC_IS_NOT_IN_LIST
	FUNC_UNKNOWN Function = -1
)

type FuncAttributes struct {
	DebugName         string
	AstName           string
	NumberOfArguments int
	NamedArguments    []string
}

var FuncAttributesMap = map[Function]FuncAttributes{
	FUNC_CONSTANT: {
		DebugName: "CONSTANT",
		AstName:   "",
	},
	FUNC_ADD: {
		DebugName:         "FUNC_ADD",
		AstName:           "+",
		NumberOfArguments: 2,
	},
	FUNC_SUBTRACT: {
		DebugName:         "FUNC_SUBTRACT",
		AstName:           "-",
		NumberOfArguments: 2,
	},
	FUNC_MULTIPLY: {
		DebugName:         "FUNC_MULTIPLY",
		AstName:           "*",
		NumberOfArguments: 2,
	},
	FUNC_DIVIDE: {
		DebugName:         "FUNC_DIVIDE",
		AstName:           "/",
		NumberOfArguments: 2,
	},
	FUNC_GREATER: {
		DebugName:         "FUNC_GREATER",
		AstName:           ">",
		NumberOfArguments: 2,
	},
	FUNC_LESS: {
		DebugName:         "FUNC_LESS",
		AstName:           "<",
		NumberOfArguments: 2,
	},
	FUNC_EQUAL: {
		DebugName:         "FUNC_EQUAL",
		AstName:           "=",
		NumberOfArguments: 2,
	},
	FUNC_NOT: {
		DebugName:         "FUNC_NOT",
		AstName:           "Not",
		NumberOfArguments: 1,
	},
	FUNC_AND: {
		DebugName:         "FUNC_AND",
		AstName:           "And",
		NumberOfArguments: 2,
	},
	FUNC_OR: {
		DebugName:         "FUNC_OR",
		AstName:           "Or",
		NumberOfArguments: 2,
	},
	FUNC_VARIABLE:           AttributeFuncVariable.FuncAttributes,
	FUNC_DB_ACCESS:          AttributeFuncDbAccess.FuncAttributes,
	FUNC_CUSTOM_LIST_ACCESS: AttributeFuncCustomListAccess.FuncAttributes,
	FUNC_IS_IN_LIST: {
		DebugName:         "FUNC_IS_IN_LIST",
		AstName:           "IsInList",
		NumberOfArguments: 2,
	},
	FUNC_IS_NOT_IN_LIST: {
		DebugName:         "FUNC_IS_NOT_IN_LIST",
		AstName:           "IsNotInList",
		NumberOfArguments: 2,
	},
}

func (f Function) Attributes() (FuncAttributes, error) {

	if attributes, ok := FuncAttributesMap[f]; ok {
		return attributes, nil
	}

	unknown := fmt.Sprintf("Unknown function: %v", f)

	return FuncAttributes{
		DebugName: unknown,
		AstName:   unknown,
	}, fmt.Errorf(unknown)
}

func (f Function) DebugString() string {
	attributes, _ := f.Attributes()
	return attributes.DebugName
}

// ======= Constant =======

func NewNodeConstant(value any) Node {
	return Node{Function: FUNC_CONSTANT, Constant: value}
}

// ======= Variable =======

var AttributeFuncVariable = struct {
	FuncAttributes
	ArgumentVarname string
}{
	FuncAttributes: FuncAttributes{
		DebugName: "FUNC_VARIABLE",
		AstName:   "Variable",
		NamedArguments: []string{
			"varname",
		},
	},
	ArgumentVarname: "varname",
}

func NewNodeVariable(varname string) Node {
	return Node{Function: FUNC_VARIABLE}.
		AddNamedChild(AttributeFuncVariable.ArgumentVarname, NewNodeConstant(varname))
}

// ======= DbAccess =======

var AttributeFuncDbAccess = struct {
	FuncAttributes
	ArgumentTableName string
	ArgumentFieldName string
	ArgumentPathName  string
}{
	FuncAttributes: FuncAttributes{
		DebugName: "FUNC_DB_ACCESS",
		AstName:   "DatabaseAccess",
		NamedArguments: []string{
			"tableName", "fieldName", "path",
		},
	},
	ArgumentTableName: "tableName",
	ArgumentFieldName: "fieldName",
	ArgumentPathName:  "path",
}

func NewNodeDatabaseAccess(tableName string, fieldName string, path []string) Node {
	return Node{Function: FUNC_DB_ACCESS}.
		AddNamedChild(AttributeFuncDbAccess.ArgumentTableName, NewNodeConstant(tableName)).
		AddNamedChild(AttributeFuncDbAccess.ArgumentFieldName, NewNodeConstant(fieldName)).
		AddNamedChild(AttributeFuncDbAccess.ArgumentPathName, NewNodeConstant(path))
}
