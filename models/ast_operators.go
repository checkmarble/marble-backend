package models

func NewASTNodePlus() *ASTNode {
	return &ASTNode{
		FuncName: "+",
	}
}

func NewASTNodeNumber(value int) *ASTNode {
	return &ASTNode{
		Constant: value,
	}
}

func NewASTNodeString(value string) *ASTNode {
	return &ASTNode{
		Constant: value,
	}
}

func NewASTSuperior() *ASTNode {
	return &ASTNode{
		FuncName: ">",
	}
}

func NewASTNodeDatabaseAccess(tableName string, fieldName string) *ASTNode {
	node := &ASTNode{
		FuncName: "DatabaseAccess",
	}

	node.AddNamedChild("tableName", NewASTNodeString(tableName))
	node.AddNamedChild("fieldName", NewASTNodeString(fieldName))
	return node
}
