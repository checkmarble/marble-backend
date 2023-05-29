package ast

import (
	"fmt"
)

type Function int

const (
	FUNC_CONSTANT Function = iota
	FUNC_PLUS
	FUNC_MINUS
	FUNC_GREATER
	FUNC_LESS
	FUNC_EQUAL
	FUNC_READ_PAYLOAD
	FUNC_DB_ACCESS
)

func (f Function) DebugString() string {
	switch f {
	case FUNC_CONSTANT:
		return "CONSTANT"
	case FUNC_PLUS:
		return "FUNC_PLUS"
	case FUNC_MINUS:
		return "FUNC_MINUS"
	case FUNC_GREATER:
		return "FUNC_GREATER"
	case FUNC_LESS:
		return "FUNC_LESS"
	case FUNC_EQUAL:
		return "FUNC_EQUAL"
	case FUNC_READ_PAYLOAD:
		return "FUNC_READ_PAYLOAD"
	case FUNC_DB_ACCESS:
		return "FUNC_DB_ACCESS"
	default:
		return fmt.Sprintf("Invalid function: %d", f)
	}
}

func NewNodeDatabaseAccess(tableName string, fieldName string) Node {
	return Node{Function: FUNC_DB_ACCESS}.
		AddNamedChild("tableName", Node{Constant: tableName}).
		AddNamedChild("fieldName", Node{Constant: fieldName})
}

func NewNodeReadPayload(fieldName string) Node {
	return Node{Function: FUNC_READ_PAYLOAD}.
		AddNamedChild(FUNC_READ_PAYLOAD_ARGUMENT_FIELD_NAME, Node{Constant: fieldName})
}

const FUNC_READ_PAYLOAD_ARGUMENT_FIELD_NAME = "fieldName"
