package models

import "github.com/checkmarble/marble-backend/models/ast"

// Define where to put that
type FilterWithType struct {
	Filter    ast.Filter
	FieldType DataType
}
