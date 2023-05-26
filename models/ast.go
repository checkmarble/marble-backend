package models

type ASTNodeType int

type AnyValue interface{}

type ASTNode struct {
	FuncName      string              `json:"funcName,omitempty"`
	Constant      AnyValue            `json:"constant,omitempty"`
	Children      []*ASTNode          `json:"children,omitempty"`
	NamedChildren map[string]*ASTNode `json:"named_children,omitempty"`
}

func (node *ASTNode) AddChild(child *ASTNode) *ASTNode {
	node.Children = append(node.Children, child)
	return node
}

func (node *ASTNode) AddNamedChild(name string, child *ASTNode) *ASTNode {
	if node.NamedChildren == nil {
		node.NamedChildren = make(map[string]*ASTNode)
	}
	node.NamedChildren[name] = child
	return node
}
